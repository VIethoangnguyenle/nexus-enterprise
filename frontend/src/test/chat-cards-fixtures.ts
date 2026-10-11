/** Fixtures for the chat cards: polls as the server answers them, tasks, a document, an approval request. */
import type { ChatTask } from '../api/messaging'
import type { PollView } from '../api/chat-cards'
import type { TextDocument } from '../api/documents'
import type { ApprovalRequestDetail } from '../api/approval'
import { U } from './chat-fixtures'

export const POLL_ID = '55555555-aaaa-4bbb-8ccc-000000000001'
export const OPT = {
  a: '66666666-aaaa-4bbb-8ccc-000000000001',
  b: '66666666-aaaa-4bbb-8ccc-000000000002',
  c: '66666666-aaaa-4bbb-8ccc-000000000003',
}

const option = (id: string, text: string, position: number, extra: Partial<PollView['options'][number]> = {}) => ({
  id, text, position, vote_count: 0, ...extra,
})

/** A poll nobody has voted in as seen by a viewer who has not voted: no counts, no voters. */
export const pollUnvoted = (over: Partial<PollView> = {}): PollView => ({
  id: POLL_ID,
  message_id: '77777777-aaaa-4bbb-8ccc-000000000001',
  channel_id: '33333333-aaaa-4bbb-8ccc-000000000001',
  question: 'Chốt lịch họp đối soát với ngân hàng?',
  options: [option(OPT.a, 'Thứ Ba 14/10, 14:00', 0), option(OPT.b, 'Thứ Tư 15/10, 9:00', 1), option(OPT.c, 'Thứ Năm 16/10, 15:30', 2)],
  is_multi: false,
  is_anonymous: false,
  created_by: U.duc.id,
  total_votes: 0,
  results_visible: false,
  my_option_ids: [],
  closed: false,
  created_at: '2026-10-11T03:31:00Z',
  ...over,
})

/** The same poll after the viewer chose `mine`: counts and voters show. */
export const pollVoted = (mine: string[] = [OPT.a], over: Partial<PollView> = {}): PollView =>
  pollUnvoted({
    results_visible: true,
    my_option_ids: mine,
    total_votes: 3,
    options: [
      option(OPT.a, 'Thứ Ba 14/10, 14:00', 0, { vote_count: 2, voter_ids: [U.hoa.id, U.lan.id] }),
      option(OPT.b, 'Thứ Tư 15/10, 9:00', 1, { vote_count: 1, voter_ids: [U.yen.id] }),
      option(OPT.c, 'Thứ Năm 16/10, 15:30', 2, { vote_count: 0, voter_ids: [] }),
    ],
    ...over,
  })

export const TASK_ID = '88888888-aaaa-4bbb-8ccc-000000000001'

export const task = (over: Partial<ChatTask> = {}): ChatTask => ({
  id: TASK_ID,
  message_id: '77777777-aaaa-4bbb-8ccc-000000000002',
  channel_id: '33333333-aaaa-4bbb-8ccc-000000000001',
  title: 'Gửi danh sách 3 giao dịch lệch cho ngân hàng',
  assignee_id: U.lan.id,
  assignee_name: 'Nguyễn Thu Lan',
  status: 'open',
  due_date: '2026-10-15',
  created_by: U.duc.id,
  created_at: '2026-10-11T03:45:00Z',
  updated_at: '2026-10-11T03:45:00Z',
  ...over,
})

export const DOC_ID = '99999999-aaaa-4bbb-8ccc-000000000001'

export const textDoc = (over: Partial<TextDocument> = {}): TextDocument => ({
  id: DOC_ID,
  workspace_id: '0a8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b',
  title: 'Biên bản họp với ngân hàng 14/10',
  version: 3,
  status: 'draft',
  owner_id: U.lan.id,
  owner_name: 'Nguyễn Thu Lan',
  created_at: '2026-10-11T04:00:00Z',
  updated_at: '2026-10-11T04:01:00Z',
  ...over,
})

export const REQ_ID = 'aaaaaaaa-aaaa-4bbb-8ccc-000000000001'

export const approvalDetail = (over: Partial<ApprovalRequestDetail['request']> = {}): ApprovalRequestDetail => ({
  request: {
    id: REQ_ID,
    entity_type: 'payment',
    entity_id: '',
    template_id: 'bbbbbbbb-aaaa-4bbb-8ccc-000000000001',
    template_name: 'Thanh toán phí liên ngân hàng tháng 10',
    form_data_json: null,
    current_step: 1,
    status: 'pending',
    scope_oa_id: '',
    department_id: '',
    created_by: U.hoa.id,
    created_by_name: 'Lê Thị Hoa',
    created_at: '2026-10-11T04:20:00Z',
    completed_at: null,
    ...over,
  },
  steps: [
    { id: 'cccccccc-aaaa-4bbb-8ccc-000000000001', step_order: 1, name: 'Trưởng phòng', approver_type: 'specific_user', approver_value: '', approver_name: 'Trần Minh Đức', required_count: 1, timeout_hours: 0 },
  ],
  form_fields: [],
  assignments: [],
  can_act: false,
})
