// Тонкий клиент к JSON-API бэкенда. Авторизация — httpOnly-cookie,
// поэтому ничего не храним: 401 означает «иди на /login».

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      /* тело не JSON — оставляем statusText */
    }
    throw new ApiError(res.status, message)
  }
  return res.json() as Promise<T>
}

export interface User {
  id: number
  email: string
  full_name: string
  initials: string
  role: 'admin' | 'inspector'
  avatar_url: string
}

export interface ActCard {
  id: number
  act_number: string
  address: string
  owner_name: string
  date: string // «23 июля»
  status: 'draft' | 'completed'
  inspector: string
  total_area: number
  rooms: number
  filled: number
  percent: number
  defects: number
  photos: number
  cloud_state: '' | 'queue' | 'error' | 'ok'
  cloud_n: number
}

export interface InspectionsPage {
  drafts: ActCard[]
  completed: ActCard[]
  draft_count: number
  completed_count: number
  page: number
  total_pages: number
}

export interface PhotoRef {
  id: number
  status: string // pending | uploading | done | failed
}

export interface Defect {
  id: number
  section: string
  section_name: string
  name: string
  value: string
  wall_number: number
  notes: string
  photos: PhotoRef[]
}

export interface Room {
  id: number
  number: number
  name: string
  defects: Defect[]
}

export interface ArchivedDefect {
  room_name: string
  name: string
  value: string
  photos: PhotoRef[]
}

export interface DocumentRef {
  id: number
  format: string
  created: string
}

export interface InspectionDetail {
  id: number
  act_number: string
  status: 'draft' | 'completed'
  date: string
  time: string
  address: string
  owner_name: string
  developer_rep_name: string
  inspector: string
  rooms_count: number
  floor: number
  total_area: number
  temp_outside: number
  temp_inside: number
  humidity: number
  electricity: string
  ventilation: string
  general_notes: string
  plan_image: string
  photo_folder_url: string
  rooms: Room[]
  archived: ArchivedDefect[]
  documents: DocumentRef[]
}

export const api = {
  login: (email: string, password: string) =>
    request<{ user: User }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  logout: () => request<{ ok: boolean }>('/api/logout', { method: 'POST' }),
  me: () => request<{ user: User }>('/api/me'),
  inspections: (params: { q?: string; page?: number }) => {
    const search = new URLSearchParams()
    if (params.q) search.set('q', params.q)
    if (params.page && params.page > 1) search.set('page', String(params.page))
    const qs = search.toString()
    return request<InspectionsPage>(`/api/inspections${qs ? `?${qs}` : ''}`)
  },
  inspection: (id: number) =>
    request<{ inspection: InspectionDetail }>(`/api/inspections/${id}`),
  // Старый обработчик отвечает redirect'ом на HTML-страницу — ответ не читаем,
  // после вызова инвалидируем детали, чтобы подтянулись новые документы
  generatePdf: async (id: number) => {
    await fetch(`/inspections/${id}/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: 'format=pdf',
    })
  },
}
