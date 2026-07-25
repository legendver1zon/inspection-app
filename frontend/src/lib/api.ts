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

export interface DefectTemplate {
  id: number
  section: string
  name: string
  threshold: string
  unit: string
}

export interface EditDefect {
  id: number
  template_id: number | null
  section: string
  value: string
  wall_number: number
  notes: string
  photos: PhotoRef[]
}

export interface EditRoomData {
  number: number
  name: string
  length: number
  width: number
  height: number
  w1h: number; w1w: number
  w2h: number; w2w: number
  w3h: number; w3w: number
  w4h: number; w4w: number
  w5h: number; w5w: number
  dh: number; dw: number
  window_type: string
  wall_types: string[]
  defects: EditDefect[]
}

export interface EditData {
  act: {
    id: number
    act_number: string
    status: string
    date: string
    time: string
    address: string
    owner_name: string
    developer_rep_name: string
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
  }
  rooms: EditRoomData[]
  templates: DefectTemplate[]
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
  editData: (id: number) => request<EditData>(`/api/inspections/${id}/edit-data`),
  checkActNumber: (id: number, value: string) =>
    request<{ taken: boolean; other_id?: number }>(
      `/api/inspections/${id}/check-act-number?value=${encodeURIComponent(value)}`,
    ),
  // Сохранение — в проверенный годами обработчик HTML-формы.
  // Ответ: redirect на просмотр (успех) или на форму с ?error= (валидация).
  saveAct: async (id: number, fields: URLSearchParams): Promise<string | null> => {
    // Обработчик ждёт multipart/form-data (ParseMultipartForm) —
    // boundary выставит браузер, заголовок не задаём
    const fd = new FormData()
    fields.forEach((v, k) => fd.append(k, v))
    const res = await fetch(`/inspections/${id}/edit`, { method: 'POST', body: fd })
    const url = new URL(res.url, window.location.origin)
    return url.searchParams.get('error')
  },
  // Загрузка фото дефекта: XHR ради прогресса отправки (у fetch его нет)
  uploadPhoto: (defectId: number, file: File, onProgress?: (pct: number) => void) =>
    new Promise<PhotoRef>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/defects/${defectId}/photos`)
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
      }
      xhr.onload = () => {
        try {
          const body = JSON.parse(xhr.responseText)
          if (xhr.status === 200) resolve({ id: body.id, status: 'pending' })
          else reject(new ApiError(xhr.status, body.error ?? 'Ошибка загрузки'))
        } catch {
          reject(new ApiError(xhr.status, 'Ошибка загрузки'))
        }
      }
      xhr.onerror = () => reject(new ApiError(0, 'Сеть недоступна'))
      const fd = new FormData()
      fd.append('photo', file)
      xhr.send(fd)
    }),
  deletePhoto: (photoId: number) =>
    request<{ ok: boolean }>(`/photos/${photoId}/delete`, { method: 'POST' }),
  uploadPlan: async (id: number, blob: Blob) => {
    const fd = new FormData()
    fd.append('plan_image', blob, 'plan.jpg')
    await fetch(`/inspections/${id}/upload-plan`, { method: 'POST', body: fd })
  },
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
