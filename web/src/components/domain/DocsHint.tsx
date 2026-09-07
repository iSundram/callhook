// DocsHint — a contextual docs link shown at stuck moments:
// errors, empty states, first-run, and "what is this?" tooltips.
import { docFromBody, docForError, DOCS } from '../../lib/docs'

export function DocsHint({ label, url, small }: { label: string; url: string; small?: boolean }) {
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener"
      className="faint"
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 5,
        fontSize: small ? 11.5 : 12.5,
        marginTop: 8,
        textDecoration: 'underline dotted',
        textUnderlineOffset: 3,
      }}
    >
      {label}
      <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        <polyline points="15 3 21 3 21 9" />
        <line x1="10" y1="14" x2="21" y2="3" />
      </svg>
    </a>
  )
}

// Shown under any error pill: "Read the fix →"
export function ErrorDocsHint({ message, body }: { message: string; body?: any }) {
  const url = docFromBody(body) || docForError(message) || DOCS.troubleshooting
  return <DocsHint label="Read the fix →" url={url} />
}
