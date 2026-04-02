import type { ReactNode } from 'react'
import { Layers } from 'lucide-react'
import type { ZoneListItem } from '../api'

type Props = {
  zones: ZoneListItem[]
  loading: boolean
  emptyTitle: string
  emptyHint: ReactNode
  onSelectRow: (z: ZoneListItem) => void
}

/** Same table chrome as Configurations / Prompts: header + scroll body, empty/loading centered. */
export function ZonesPickerTable({ zones, loading, emptyTitle, emptyHint, onSelectRow }: Props) {
  return (
    <div className="table-wrap table-wrap-logs zones-picker-table">
      <div className="table-header-wrap">
        <table className="data-table data-table-header">
          <thead>
            <tr>
              <th style={{ width: '40%' }}>Name</th>
              <th style={{ width: '35%' }}>ID</th>
              <th style={{ width: '25%' }}>Agent</th>
            </tr>
          </thead>
        </table>
      </div>
      <div className="table-body-wrap">
        {loading ? (
          <div className="table-empty">
            <p className="text-muted zones-table-loading">Loading…</p>
          </div>
        ) : zones.length === 0 ? (
          <div className="table-empty">
            <p className="table-empty-msg">{emptyTitle}</p>
            <p className="text-muted table-empty-hint">{emptyHint}</p>
          </div>
        ) : (
          <table className="data-table data-table-body">
            <tbody>
              {zones.map((z) => (
                <tr
                  key={z.id}
                  className="chat-log-row zones-row"
                  onClick={() => onSelectRow(z)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => e.key === 'Enter' && onSelectRow(z)}
                >
                  <td style={{ width: '40%' }}>
                    <span className="zones-name">
                      <Layers className="zones-row-icon" size={18} aria-hidden />
                      {z.name}
                    </span>
                  </td>
                  <td style={{ width: '35%' }} className="mono">
                    {z.id}
                  </td>
                  <td style={{ width: '25%' }}>
                    <span className={z.agent_ok ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--bad'}>
                      {z.agent_ok ? 'reachable' : 'unreachable'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
