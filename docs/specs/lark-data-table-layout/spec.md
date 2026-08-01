# lark-data-table-layout

## Purpose
Give list-shaped modules one consistent tabular presentation, so Drive and Assets do not each invent their own.

## Status

Verified against code 2026-07-31. **Partially implemented.**

Row-based rendering exists (`DriveFileList.tsx`, `DriveFileRow.tsx`). Breadcrumb navigation was
found only in `routes/_workspace/contacts.tsx`, not in Drive, so the breadcrumb requirement is
unmet where this spec most calls for it.

**Bổ sung 2026-08-01 — di cư design system.** Drive và Assets đã dùng chung token và primitive
(`Button`, `IconButton`, `Input`, `Badge`), nên phần "một cách trình bày nhất quán" giờ được
cưỡng chế bằng lint chứ không còn là quy ước. **Yêu cầu breadcrumb vẫn chưa được đáp ứng** —
đợt di cư này chỉ đụng tới lớp trình bày, không thêm năng lực điều hướng nào.

Bốn chỗ trong `components/drive` giữ số đo pixel tuỳ tiện kèm chú thích miễn trừ: chúng là
chiều rộng cột của bảng, phải khớp nhau theo pixel giữa hàng tiêu đề và hàng dữ liệu, và thang
spacing 4px không diễn đạt được ràng buộc đó. Xem `docs/superpowers/specs/2026-08-01-ui-design-system-enforcement-design.md` §1.3.

## Requirements

### Requirement: Standard Data Table
Drive and Assets modules SHALL use a common data-table layout for displaying items, replacing grid and card views. The table SHALL have a header row with column titles, and flat border separators between rows.

#### Scenario: Drive displays files as a table
- **WHEN** user views files in Drive
- **THEN** files SHALL be listed in a structured data table

### Requirement: Table Row Interactions
Data table rows SHALL NOT use box shadows or translate transforms on hover. They SHALL use a flat background color change (e.g., `bg-bg-hover`) to indicate interactivity.

#### Scenario: Hovering a file row
- **WHEN** user hovers over a table row
- **THEN** the row background SHALL darken slightly without elevation

### Requirement: Breadcrumb Navigation
Hierarchical data (like Drive folders) SHALL use breadcrumb navigation at the top of the view rather than inline "Up" buttons.

#### Scenario: Navigating folders
- **WHEN** user navigates deep into a folder structure
- **THEN** a breadcrumb trail SHALL appear allowing navigation back to parent folders

### Requirement: Peek Panel Details
Item details (e.g., file metadata, asset history) SHALL be displayed in a right-side peek panel rather than a centered modal dialog.

#### Scenario: Viewing item details
- **WHEN** user clicks to view details of an item
- **THEN** a side panel SHALL slide in from the right over the content area
