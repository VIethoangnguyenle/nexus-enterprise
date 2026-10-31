/**
 * Why push cannot work on this browser, and what to do: a browser that blocks Nexus gets the three steps to allow it;
 * an iPhone, where push needs Nexus on the home screen, is told to add it.
 */
export function PushBanners({ blocked, ios }: { blocked: boolean; ios: boolean }) {
  return (
    <>
      {blocked && (
        <div
          role="status"
          className="p-3.5 rounded-xl bg-danger-wash text-ink border border-danger/20 text-xs flex flex-col gap-2"
        >
          <strong className="font-semibold text-danger">Trình duyệt đang chặn thông báo của Nexus</strong>
          <ol className="list-decimal pl-4 space-y-1 text-ink-muted">
            <li>Bấm biểu tượng ổ khoá cạnh địa chỉ trang.</li>
            <li>Chọn Quyền → Thông báo → Cho phép.</li>
            <li>Quay lại đây, công tắc sẽ tự bật.</li>
          </ol>
        </div>
      )}

      {ios && (
        <div
          role="status"
          className="p-3.5 rounded-xl bg-info-wash text-ink border border-info/20 text-xs flex flex-col gap-1.5"
        >
          <strong className="font-semibold text-info">
            Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone.
          </strong>
        </div>
      )}
    </>
  )
}
