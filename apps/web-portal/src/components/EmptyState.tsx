import type { ReactNode } from "react";

export function EmptyState({ title, body, icon }: { title: string; body: string; icon?: ReactNode }) {
  return (
    <div className="empty-state">
      <div className="empty-icon">
        {icon ?? (
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
            <path d="M4 7h16M4 12h10M4 17h14" strokeLinecap="round" />
          </svg>
        )}
      </div>
      <strong>{title}</strong>
      <p>{body}</p>
    </div>
  );
}
