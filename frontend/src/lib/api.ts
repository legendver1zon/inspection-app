// Тонкий клиент к JSON-API бэкенда. Авторизация — httpOnly-cookie,
// поэтому ничего не храним: 401 означает «иди на /login».

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export const AUTH_EXPIRED_EVENT = 'auth:expired'

// Старые обработчики форм отвечают редиректом; при истёкшей сессии — 401 JSON
function guard401(res: Response) {
  if (res.status === 401) {
    window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT))
    throw new ApiError(401, 'Сессия истекла, войдите заново')
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest', ...init?.headers },
    ...init,
  })
  if (res.status === 401 && !path.startsWith('/api/login') && !path.startsWith('/api/me')) {
    window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT))
  }
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

// Фото к общим замечаниям по квартире
export interface GeneralPhotos {
  electricity: PhotoRef[]
  ventilation: PhotoRef[]
  general: PhotoRef[]
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
  photos: PhotoRef[] // общий вид помещения
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
  general_photos: GeneralPhotos
  can_delete: boolean
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
  photos: PhotoRef[]
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
    general_photos: GeneralPhotos
  }
  rooms: EditRoomData[]
  templates: DefectTemplate[]
}

export interface DashboardStats {
  total: number
  draft: number
  completed: number
  today: number
  week: number
  photo_pending: number
  photo_failed: number
}

export interface AdminUser extends User {
  created: string
  acts: number
}

export const api = {
  login: (email: string, password: string) =>
    request<{ user: User }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  logout: async () => {
    const r = await request<{ ok: boolean }>('/api/logout', { method: 'POST' })
    try { localStorage.removeItem('me') } catch { /* ignore */ }
    return r
  },
  // Без сети сервер недоступен, но сессия жива: берём пользователя из
  // последнего успешного ответа, чтобы не выкидывать на вход
  me: async () => {
    try {
      const r = await request<{ user: User }>('/api/me')
      try { localStorage.setItem('me', JSON.stringify(r.user)) } catch { /* ignore */ }
      return r
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.status === 401) { try { localStorage.removeItem('me') } catch { /* ignore */ } }
        throw e
      }
      let cached: string | null = null
      try { cached = localStorage.getItem('me') } catch { /* ignore */ }
      if (cached) return { user: JSON.parse(cached) as User }
      throw e
    }
  },
  register: (body: {
    email: string
    password: string
    confirm_password: string
    full_name: string
    no_patronymic: boolean
  }) => request<{ ok: boolean }>('/api/register', { method: 'POST', body: JSON.stringify(body) }),
  forgotPassword: (email: string) =>
    request<{ ok: boolean }>('/api/forgot-password', { method: 'POST', body: JSON.stringify({ email }) }),
  resetPassword: (body: { email: string; code: string; password: string; confirm: string }) =>
    request<{ ok: boolean }>('/api/reset-password', { method: 'POST', body: JSON.stringify(body) }),
  inspections: (params: { q?: string; page?: number }) => {
    const search = new URLSearchParams()
    if (params.q) search.set('q', params.q)
    if (params.page && params.page > 1) search.set('page', String(params.page))
    const qs = search.toString()
    return request<InspectionsPage>(`/api/inspections${qs ? `?${qs}` : ''}`)
  },
  inspection: (id: number) =>
    request<{ inspection: InspectionDetail }>(`/api/inspections/${id}`),
  createInspection: () => request<{ id: number; act_number: string }>('/api/inspections', { method: 'POST' }),
  deleteInspection: async (id: number) => {
    const res = await fetch(`/inspections/${id}/delete`, { method: 'POST', headers: { 'X-Requested-With': 'XMLHttpRequest' } })
    guard401(res)
    if (!res.ok) {
      let message = 'Не удалось удалить акт'
      try { message = (await res.json()).error ?? message } catch { /* не JSON */ }
      throw new ApiError(res.status, message)
    }
  },
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
    const res = await fetch(`/inspections/${id}/edit`, { method: 'POST', body: fd, headers: { 'X-Requested-With': 'XMLHttpRequest' } })
    guard401(res)
    const url = new URL(res.url, window.location.origin)
    return url.searchParams.get('error')
  },
  // Загрузка фото дефекта: XHR ради прогресса отправки (у fetch его нет)
  uploadPhoto: (defectId: number, blob: Blob, name: string, onProgress?: (pct: number) => void, timeoutMs = 120_000) =>
    new Promise<PhotoRef>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/defects/${defectId}/photos`)
      xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest')
      xhr.timeout = timeoutMs
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
      }
      xhr.onload = () => {
        if (xhr.status === 401) window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT))
        try {
          const body = JSON.parse(xhr.responseText)
          if (xhr.status === 200) resolve({ id: body.id, status: 'pending' })
          else reject(new ApiError(xhr.status, body.error ?? 'Ошибка загрузки'))
        } catch {
          reject(new ApiError(xhr.status, xhr.status === 413 ? 'Файл слишком большой' : 'Ошибка загрузки'))
        }
      }
      xhr.onerror = () => reject(new ApiError(0, 'Сеть недоступна'))
      xhr.ontimeout = () => reject(new ApiError(408, 'Слишком долгая отправка, попробуем ещё раз'))
      const fd = new FormData()
      fd.append('photo', blob, name)
      xhr.send(fd)
    }),
  // Загрузка по ключу помещение/раздел/шаблон/стена: id дефекта меняется при
  // каждом сохранении формы, а ключ стабилен; client_id защищает от дублей
  uploadPhotoByKey: (
    k: { actId: number; roomNumber: number; section: string; templateId: number | null; wallNumber: number },
    blob: Blob,
    name: string,
    clientId: string,
    onProgress?: (pct: number) => void,
    timeoutMs = 120_000,
  ) =>
    new Promise<{ photo: PhotoRef; defectId: number }>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/inspections/${k.actId}/photos`)
      xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest')
      xhr.timeout = timeoutMs
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
      }
      xhr.onload = () => {
        if (xhr.status === 401) window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT))
        try {
          const body = JSON.parse(xhr.responseText)
          if (xhr.status === 200) resolve({ photo: { id: body.id, status: 'pending' }, defectId: body.defect_id ?? 0 })
          else reject(new ApiError(xhr.status, body.error ?? 'Ошибка загрузки'))
        } catch {
          reject(new ApiError(xhr.status, xhr.status === 413 ? 'Файл слишком большой' : 'Ошибка загрузки'))
        }
      }
      xhr.onerror = () => reject(new ApiError(0, 'Сеть недоступна'))
      xhr.ontimeout = () => reject(new ApiError(408, 'Слишком долгая отправка, попробуем ещё раз'))
      const fd = new FormData()
      fd.append('room_number', String(k.roomNumber))
      fd.append('section', k.section)
      fd.append('template_id', k.templateId == null ? '' : String(k.templateId))
      fd.append('wall_number', String(k.wallNumber))
      fd.append('client_id', clientId)
      fd.append('photo', blob, name)
      xhr.send(fd)
    }),
  deletePhoto: (photoId: number) =>
    request<{ ok: boolean }>(`/photos/${photoId}/delete`, { method: 'POST' }),
  dashboard: () => request<DashboardStats>('/api/dashboard'),
  updateProfile: (body: {
    full_name: string
    initials: string
    current_password?: string
    new_password?: string
    confirm?: string
  }) => request<{ user: User }>('/api/profile', { method: 'POST', body: JSON.stringify(body) }),
  uploadAvatar: async (file: File) => {
    const fd = new FormData()
    fd.append('avatar', file)
    guard401(await fetch('/profile/avatar', { method: 'POST', body: fd, headers: { 'X-Requested-With': 'XMLHttpRequest' } }))
  },
  users: () => request<{ users: AdminUser[] }>('/api/users'),
  updateUser: (id: number, body: { full_name: string; email: string; role: string; new_password?: string }) =>
    request<{ user: User }>(`/api/users/${id}`, { method: 'POST', body: JSON.stringify(body) }),
  deleteUser: (id: number) =>
    request<{ ok: boolean }>(`/api/users/${id}/delete`, { method: 'POST' }),
  setStatus: async (id: number, status: 'draft' | 'completed') => {
    guard401(await fetch(`/inspections/${id}/status`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'X-Requested-With': 'XMLHttpRequest' },
      body: `status=${status}`,
    }))
  },
  uploadPlan: async (id: number, blob: Blob) => {
    const fd = new FormData()
    fd.append('plan_image', blob, 'plan.jpg')
    guard401(await fetch(`/inspections/${id}/upload-plan`, { method: 'POST', body: fd, headers: { 'X-Requested-With': 'XMLHttpRequest' } }))
  },
  deleteDocument: async (id: number) => {
    guard401(await fetch(`/documents/${id}/delete`, { method: 'POST', headers: { 'X-Requested-With': 'XMLHttpRequest' } }))
  },
  // Старый обработчик отвечает redirect'ом на HTML-страницу — ответ не читаем,
  // после вызова инвалидируем детали, чтобы подтянулись новые документы
  generatePdf: async (id: number) => {
    guard401(await fetch(`/inspections/${id}/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'X-Requested-With': 'XMLHttpRequest' },
      body: 'format=pdf',
    }))
  },
}
