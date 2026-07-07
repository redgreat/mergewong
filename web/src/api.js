const baseUrl = import.meta.env.VITE_API_BASE || "";

export async function request(path, options = {}) {
  const { method = "GET", body, token, params } = options;
  let url = `${baseUrl}${path}`;
  const origin = globalThis.location?.origin;

  if (origin) {
    url = new URL(url, origin).toString();
  }

  if (params) {
    const search = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== "") {
        search.set(key, String(value));
      }
    });
    const qs = search.toString();
    if (qs) url += `?${qs}`;
  }

  const headers = {
    "Content-Type": "application/json"
  };

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(url, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined
  });

  const payload = await response.json().catch(() => null);

  if (!response.ok) {
    throw new Error(payload?.message || response.statusText);
  }

  if (payload && payload.code && payload.code !== 200) {
    throw new Error(payload.message || "请求失败");
  }

  return payload?.data ?? payload;
}
