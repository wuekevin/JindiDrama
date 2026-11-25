// Wails 的自定义 WebView 协议下也可生成本地互动 ID。
export function localId() {
  return Array.from(crypto.getRandomValues(new Uint32Array(4)), (part) => part.toString(16).padStart(8, '0')).join('')
}
