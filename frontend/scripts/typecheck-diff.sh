#!/usr/bin/env bash
# So lỗi TypeScript theo TỪNG FILE với baseline đã ghi.
#
# Vì sao không so tổng số: dự án có 123 lỗi type có sẵn (chưa từng được
# typecheck cho tới 2026-08-01), nên cổng "0 lỗi" là bất khả thi và cổng
# "tổng không tăng" thì che mất việc một file sạch bắt đầu hỏng.
#
# Vì sao đếm theo file chứ không so từng dòng lỗi: số dòng xê dịch mỗi khi
# sửa file, sẽ sinh ra khác biệt giả.
set -uo pipefail
cd "$(dirname "$0")/.."

BASELINE=typecheck-baseline.txt
CURRENT=$(mktemp)
trap 'rm -f "$CURRENT"' EXIT

npx tsc --noEmit -p tsconfig.json 2>&1 \
  | grep -oE '^[^(]+\.tsx?' \
  | sort | uniq -c \
  | awk '{print $2"\t"$1}' | sort > "$CURRENT"

if [ ! -f "$BASELINE" ]; then
  echo "Chưa có $BASELINE. Chạy: cp $CURRENT $BASELINE"
  cat "$CURRENT"
  exit 1
fi

fail=0
while IFS=$'\t' read -r file count; do
  [ -z "$file" ] && continue
  base=$(awk -F'\t' -v f="$file" '$1 == f { print $2 }' "$BASELINE")
  base=${base:-0}
  if [ "$count" -gt "$base" ]; then
    echo "LỖI TYPE MỚI  $file: $count (baseline $base)"
    fail=1
  fi
done < "$CURRENT"

if [ "$fail" -eq 0 ]; then
  echo "typecheck: không file nào tăng lỗi so với baseline"
fi
exit "$fail"
