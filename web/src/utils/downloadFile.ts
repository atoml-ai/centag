export type DownloadResult =
  | { mode: 'browser'; filename: string }
  | { mode: 'cancelled' }

function triggerBrowserDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

/** Save a blob via browser download. */
export async function saveBlobAsFile(blob: Blob, filename: string): Promise<DownloadResult> {
  if (!blob || blob.size === 0) {
    throw new Error('导出内容为空')
  }
  triggerBrowserDownload(blob, filename)
  return { mode: 'browser', filename }
}
