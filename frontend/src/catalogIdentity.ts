export function isAppleCMSDramaId(id: string) {
  return /^apple-cms:[0-9a-f]{32}:[0-9]{1,30}$/.test(id)
}
