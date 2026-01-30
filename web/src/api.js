const baseUrl = import.meta.env.VITE_API_BASE || "http://localhost:8080";

export async function request(path, options = {}) {
  const { method = "GET", body, token, params } = options;
  const url = new URL(`${baseUrl}${path}`);

  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== "") {
        url.searchParams.set(key, String(value));
      }
    });
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
