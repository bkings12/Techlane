const STAR = "★";

/** Read-only star display — only rendered when real reviews exist upstream. */
export function StarRating({ value, count }: { value: number; count?: number }) {
  const rounded = Math.round(value);
  return (
    <span className="star-rating" aria-label={`${value.toFixed(1)} out of 5 stars`}>
      <span className="star-rating-stars" aria-hidden="true">
        {Array.from({ length: 5 }, (_, i) => (
          <span key={i} className={i < rounded ? "star-on" : "star-off"}>
            {STAR}
          </span>
        ))}
      </span>
      {count != null ? <span className="star-rating-count">({count})</span> : null}
    </span>
  );
}

/** Interactive star picker for the review submission form. */
export function StarRatingInput({ value, onChange }: { value: number; onChange: (next: number) => void }) {
  return (
    <span className="star-rating star-rating-input" role="radiogroup" aria-label="Rating">
      {Array.from({ length: 5 }, (_, i) => {
        const n = i + 1;
        return (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={value === n}
            aria-label={`${n} star${n > 1 ? "s" : ""}`}
            className={n <= value ? "star-on" : "star-off"}
            onClick={() => onChange(n)}
          >
            {STAR}
          </button>
        );
      })}
    </span>
  );
}
