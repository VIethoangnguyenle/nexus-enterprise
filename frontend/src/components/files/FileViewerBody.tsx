import type { UseQueryResult } from '@tanstack/react-query'
import { AlertTriangle, Download } from 'lucide-react'
import { ApiError } from '../../api/client'
import type { FilePreview, PreviewUnavailableReason } from '../../api/drive'
import { fileKind } from '../../lib/file-kind'
import { formatBytes } from '../../lib/format'
import { Button } from '../primitives'
import { ImageBody, type ImageZoom } from './ImageBody'
import { SheetBody } from './SheetBody'
import { TextBody } from './TextBody'
import {
  DeniedState, DownloadButton, GoneState, NetworkState, PreparingState, SkeletonStage, StateBlock,
} from './ViewerStates'
import { isConvertible, officeLabel } from './viewer-text'
import type { ViewerFile } from './FileViewer'

/** What a failed preview request means for the viewer. */
export type PreviewFailure = 'denied' | 'gone' | 'network'

export function failureOf(error: unknown): PreviewFailure {
  if (error instanceof ApiError && error.status === 403) return 'denied'
  if (error instanceof ApiError && error.status === 404) return 'gone'
  return 'network'
}

const REASON: Record<Exclude<PreviewUnavailableReason, 'unsupported' | 'too_large'>, string> = {
  conversion_failed: 'Máy chủ không mở được tệp này, có thể tệp có mật khẩu hoặc bị hỏng. Tải xuống rồi mở bằng ứng dụng trên máy.',
  conversion_unavailable: 'Dịch vụ xem trước đang tạm ngưng. Tải xuống để mở bằng ứng dụng trên máy.',
  conversion_busy: 'Máy chủ đang bận chuyển các tệp khác. Thử lại sau ít phút, hoặc tải xuống để mở trên máy.',
}

function Unavailable({
  reason, file, onDownload, onRetry,
}: { reason: PreviewUnavailableReason; file: ViewerFile; onDownload: () => void; onRetry: () => void }) {
  const kind = fileKind(file.name, file.mime)
  if (reason === 'unsupported') {
    return (
      <StateBlock
        icon={kind.icon}
        title="Không xem trước được loại tệp này"
        text={[kind.label, file.size ? formatBytes(file.size) : ''].filter(Boolean).join(' · ') + '. Tải xuống để mở bằng ứng dụng trên máy.'}
      >
        <DownloadButton onClick={onDownload} />
      </StateBlock>
    )
  }
  if (reason === 'too_large') {
    return (
      <StateBlock
        icon={Download}
        tone="bad"
        title="Tệp quá lớn để xem trước"
        text={`Tệp này${file.size ? ` ${formatBytes(file.size)},` : ''} vượt giới hạn xem trước. Tải xuống để mở trên máy.`}
      >
        <DownloadButton onClick={onDownload} />
      </StateBlock>
    )
  }
  return (
    <StateBlock icon={AlertTriangle} tone="bad" title="Không tạo được bản xem trước" text={REASON[reason]}>
      <div className="flex gap-2">
        <DownloadButton onClick={onDownload} />
        {reason !== 'conversion_failed' && (
          <Button type="button" variant="soft" size="sm" onClick={onRetry}>Thử lại</Button>
        )}
      </div>
    </StateBlock>
  )
}

/**
 * The middle of the viewer for one file, chosen by how the drive says to show
 * it. The PDF the server makes (and a PDF itself) goes to the browser's own
 * viewer in a frame, so its page and zoom controls are the browser's.
 */
export function FileViewerBody({
  file, preview, zoom, onZoom, onDownload, onPrevious, onNext,
}: {
  file: ViewerFile
  preview: UseQueryResult<FilePreview, unknown>
  zoom: ImageZoom
  onZoom: (z: ImageZoom) => void
  onDownload: () => void
  onPrevious?: () => void
  onNext?: () => void
}) {
  if (preview.isError) {
    const failure = failureOf(preview.error)
    if (failure === 'denied') return <DeniedState />
    if (failure === 'gone') return <GoneState />
    return <NetworkState onRetry={() => void preview.refetch()} />
  }
  if (preview.isPending) {
    return isConvertible(file.name)
      ? <PreparingState label={officeLabel(file.name)} onDownload={onDownload} />
      : <SkeletonStage />
  }

  const p = preview.data
  switch (p.kind) {
    case 'unavailable':
      return <Unavailable reason={p.reason} file={file} onDownload={onDownload} onRetry={() => void preview.refetch()} />
    case 'sheet':
      return <SheetBody url={p.url} onDownload={onDownload} />
    case 'pdf':
      return (
        <div className="grid grid-rows-[auto_minmax(0,1fr)] h-full">
          <p className="m-0 px-4 py-2 text-small text-ink-muted text-center">
            Bản xem trước từ tệp {officeLabel(file.name)}, có thể khác bản gốc đôi chút.
          </p>
          <iframe src={p.url} title={`Bản xem trước ${file.name}`} className="w-full h-full border-0 bg-base" />
        </div>
      )
    case 'native': {
      const { url, mime } = p
      if (mime.startsWith('image/')) {
        return (
          <ImageBody
            url={url}
            name={file.name}
            zoom={zoom}
            onZoom={onZoom}
            onPrevious={onPrevious}
            onNext={onNext}
          />
        )
      }
      if (mime === 'application/pdf') {
        return <iframe src={url} title={`Bản xem trước ${file.name}`} className="w-full h-full border-0 bg-base" />
      }
      if (mime.startsWith('video/')) {
        return (
          <div className="grid place-items-center h-full p-4">
            <video src={url} controls preload="metadata" aria-label={file.name} className="max-w-full max-h-full" />
          </div>
        )
      }
      if (mime.startsWith('audio/')) {
        const kind = fileKind(file.name, mime)
        return (
          <div className="grid place-items-center h-full p-4">
            <div className="grid justify-items-center gap-3 p-6 rounded-lg bg-base max-w-90 w-full">
              <span className={`grid place-items-center w-14 h-14 rounded-lg ${kind.tile}`} aria-hidden="true">
                <kind.icon size={28} strokeWidth={1.5} />
              </span>
              <b className="text-ink text-center break-words max-w-full">{file.name}</b>
              <audio src={url} controls preload="none" aria-label={file.name} className="w-full" />
            </div>
          </div>
        )
      }
      // Every text/* (html and svg included) and JSON: fetched and shown as text, never as markup.
      return <TextBody url={url} name={file.name} mime={mime} onDownload={onDownload} />
    }
  }
}
