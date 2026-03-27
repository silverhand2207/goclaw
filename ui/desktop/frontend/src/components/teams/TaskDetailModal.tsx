import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { MarkdownRenderer } from '../chat/MarkdownRenderer'
import { Combobox } from '../common/Combobox'
import { ConfirmDialog } from '../common/ConfirmDialog'
import { STATUS_BADGE, PRIORITY_BADGE, isTaskLocked } from '../../types/team'
import type { TeamTaskData, TeamMemberData, TaskStatus } from '../../types/team'

interface TaskDetailModalProps {
  task: TeamTaskData
  members: TeamMemberData[]
  onClose: () => void
  onAssign: (taskId: string, agentKey: string) => Promise<unknown>
  onDelete: (taskId: string) => Promise<void>
}

const TERMINAL: Set<TaskStatus> = new Set(['completed', 'failed', 'cancelled'])

/** Metadata label + value pair */
function MetaItem({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-text-muted mb-0.5">{label}</dt>
      <dd className="text-sm font-medium text-text-primary">{children}</dd>
    </div>
  )
}

/** Collapsible bordered section — matches web UI TaskDetailContent pattern */
function CollapsibleSection({ title, icon, defaultOpen = true, children }: {
  title: string; icon: React.ReactNode; defaultOpen?: boolean; children: React.ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className="rounded-lg border border-border">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 px-4 py-3 text-sm font-medium text-text-muted hover:text-text-primary transition-colors cursor-pointer"
      >
        {icon}
        <span>{title}</span>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} className={`ml-auto transition-transform ${open ? '' : '-rotate-90'}`}>
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>
      {open && (
        <div className="border-t border-border px-4 py-3">
          {children}
        </div>
      )}
    </div>
  )
}

export function TaskDetailModal({ task, members, onClose, onAssign, onDelete }: TaskDetailModalProps) {
  const { t } = useTranslation('teams')
  const [confirmDelete, setConfirmDelete] = useState(false)

  const prio = PRIORITY_BADGE[task.priority] ?? PRIORITY_BADGE[3]
  const statusCls = STATUS_BADGE[task.status] ?? ''
  const locked = isTaskLocked(task)
  const isTerminal = TERMINAL.has(task.status)
  const member = task.owner_agent_id ? members.find((m) => m.agent_id === task.owner_agent_id) : undefined

  const memberOptions = members.map((m) => ({
    value: m.agent_key || m.agent_id,
    label: `${m.emoji || ''} ${m.display_name || m.agent_key || m.agent_id}`.trim(),
  }))

  const handleDelete = async () => {
    try {
      await onDelete(task.id)
      onClose()
    } finally {
      setConfirmDelete(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="bg-surface-primary border border-border rounded-xl shadow-xl w-[95vw] max-w-4xl max-h-[85vh] flex flex-col mx-4"
      >
        {/* ── Header: badges + title ── */}
        <div className="px-6 pt-5 pb-4 border-b border-border shrink-0">
          <div className="flex items-start gap-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-2">
                {task.identifier && (
                  <span className="text-xs font-mono text-text-muted bg-surface-tertiary px-2 py-0.5 rounded border border-border">
                    {task.identifier}
                  </span>
                )}
                <span className={`text-xs font-medium px-2 py-0.5 rounded capitalize ${statusCls}`}>
                  {task.status.replace(/_/g, ' ')}
                </span>
                {locked && (
                  <span className="text-xs font-medium px-2 py-0.5 rounded bg-green-500/15 text-green-600 dark:text-green-400 animate-pulse">
                    Running
                  </span>
                )}
              </div>
              <h3 className="text-base font-semibold text-text-primary leading-snug sm:text-lg">{task.subject}</h3>
            </div>
            <button onClick={onClose} className="text-text-muted hover:text-text-primary p-1.5 cursor-pointer shrink-0 rounded-lg hover:bg-surface-tertiary">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </div>

        {/* ── Scrollable body ── */}
        <div className="flex-1 overflow-y-auto overscroll-contain space-y-4 px-6 py-4">

          {/* Progress bar */}
          {task.progress_percent != null && task.progress_percent > 0 && !isTerminal && (() => {
            const pct = Math.min(100, Math.max(0, task.progress_percent))
            return (
              <div className="space-y-1">
                <div className="flex justify-between text-xs text-text-muted">
                  <span>{t('progress', 'Progress')}</span>
                  <span>{pct}%</span>
                </div>
                <div className="h-2 w-full rounded-full bg-surface-tertiary overflow-hidden">
                  <div className="h-2 rounded-full bg-accent transition-all duration-500" style={{ width: `${pct}%` }} />
                </div>
                {task.progress_step && <p className="text-xs text-text-muted">{task.progress_step}</p>}
              </div>
            )
          })()}

          {/* Metadata grid */}
          <dl className="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-3 rounded-lg bg-surface-tertiary/30 p-4">
            <MetaItem label={t('priority', 'Priority')}>
              <span className={`inline-flex items-center gap-1.5 capitalize`}>
                <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${prio.cls}`}>{prio.label}</span>
              </span>
            </MetaItem>
            <MetaItem label={t('owner', 'Owner')}>
              {member ? (
                <span className="flex items-center gap-1.5">
                  {member.emoji && <span className="text-base">{member.emoji}</span>}
                  {member.display_name || member.agent_key}
                </span>
              ) : task.owner_agent_key || '—'}
            </MetaItem>
            {task.task_type && task.task_type !== 'general' && (
              <MetaItem label={t('type', 'Type')}>
                <span className="text-xs bg-surface-tertiary border border-border px-2 py-0.5 rounded">{task.task_type}</span>
              </MetaItem>
            )}
            {task.created_at && (
              <MetaItem label={t('created', 'Created')}>
                {new Date(task.created_at).toLocaleString()}
              </MetaItem>
            )}
            {task.updated_at && (
              <MetaItem label={t('updated', 'Updated')}>
                {new Date(task.updated_at).toLocaleString()}
              </MetaItem>
            )}
          </dl>

          {/* Blocked by */}
          {task.blocked_by && task.blocked_by.length > 0 && (
            <div>
              <span className="text-xs text-text-muted">{t('blockedBy', 'Blocked by')}</span>
              <div className="mt-1 flex flex-wrap gap-1.5">
                {task.blocked_by.map((id) => (
                  <span key={id} className="text-xs font-mono bg-amber-500/15 text-amber-600 dark:text-amber-400 px-2 py-0.5 rounded border border-amber-500/20">
                    {id.slice(0, 8)}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Description section */}
          {task.description && (
            <CollapsibleSection
              title={t('description', 'Description')}
              icon={<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" /><polyline points="10 9 9 9 8 9" /></svg>}
            >
              <div className="text-sm text-text-secondary prose prose-sm dark:prose-invert max-w-none max-h-60 overflow-y-auto">
                <MarkdownRenderer content={task.description} />
              </div>
            </CollapsibleSection>
          )}

          {/* Result section */}
          {task.result && (
            <CollapsibleSection
              title={t('result', 'Result')}
              icon={<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><polyline points="22 4 12 14.01 9 11.01" /></svg>}
            >
              <div className="text-sm text-text-secondary prose prose-sm dark:prose-invert max-w-none max-h-[40vh] overflow-y-auto">
                <MarkdownRenderer content={task.result} />
              </div>
            </CollapsibleSection>
          )}
        </div>

        {/* ── Footer ── */}
        <div className="flex items-center gap-3 px-6 py-3 border-t border-border shrink-0">
          {!isTerminal && (
            <div className="max-w-[240px]">
              <Combobox
                options={memberOptions}
                value={member?.agent_key || task.owner_agent_key || ''}
                onChange={(key) => onAssign(task.id, key)}
                placeholder={t('assignTo', 'Assign to...')}
              />
            </div>
          )}
          <div className="flex-1" />
          {isTerminal && (
            <button
              onClick={() => setConfirmDelete(true)}
              className="flex items-center gap-1.5 text-sm text-error hover:text-error/80 px-4 py-2 rounded-lg border border-error/30 hover:bg-error/10 transition-colors cursor-pointer"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
                <polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
              </svg>
              {t('delete', 'Delete')}
            </button>
          )}
        </div>

        <ConfirmDialog
          open={confirmDelete}
          onOpenChange={setConfirmDelete}
          title={t('deleteTask', 'Delete task?')}
          description={t('deleteTaskConfirm', 'This action cannot be undone.')}
          confirmLabel={t('delete', 'Delete')}
          variant="destructive"
          onConfirm={handleDelete}
        />
      </div>
    </div>
  )
}
