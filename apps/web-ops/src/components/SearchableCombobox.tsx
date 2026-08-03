import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { Button, Icon, Input } from "./ui";

export type ComboOption = {
  value: string;
  label: string;
  sublabel?: string;
};

export type SearchableComboboxProps = {
  label: string;
  placeholder: string;
  options: ComboOption[];
  onSearch?: (query: string) => void;
  value: string;
  onSelect: (option: ComboOption) => void;
  onQueryChange?: (query: string) => void;
  onAddNew?: (input: { primary: string; secondary?: string }) => Promise<ComboOption>;
  addNewFields?: { primary: string; secondary?: string };
  loading?: boolean;
  disabled?: boolean;
};

export function SearchableCombobox({
  label,
  placeholder,
  options,
  onSearch,
  value,
  onSelect,
  onQueryChange,
  onAddNew,
  addNewFields,
  loading = false,
  disabled = false,
}: SearchableComboboxProps) {
  const listId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const [adding, setAdding] = useState(false);
  const [primary, setPrimary] = useState("");
  const [secondary, setSecondary] = useState("");
  const [saving, setSaving] = useState(false);
  const [addError, setAddError] = useState("");

  const selected = options.find((o) => o.value === value) ?? null;

  const filtered = useMemo(() => {
    if (onSearch) return options;
    const q = query.trim().toLowerCase();
    if (!q) return options;
    return options.filter(
      (o) =>
        o.label.toLowerCase().includes(q) ||
        (o.sublabel ?? "").toLowerCase().includes(q) ||
        o.value.toLowerCase().includes(q),
    );
  }, [options, onSearch, query]);

  useEffect(() => {
    if (!onSearch) return;
    const t = window.setTimeout(() => onSearch(query), 250);
    return () => window.clearTimeout(t);
  }, [query, onSearch]);

  const onQueryChangeRef = useRef(onQueryChange);
  onQueryChangeRef.current = onQueryChange;
  useEffect(() => {
    onQueryChangeRef.current?.(query);
  }, [query]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) {
        setOpen(false);
        setAdding(false);
      }
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  useEffect(() => {
    // Prefer the first real match over a leading "Walk-in" / empty sentinel when searching.
    const firstReal = filtered.findIndex((o) => o.value && !o.value.startsWith("__"));
    setHighlight(firstReal >= 0 ? firstReal : 0);
  }, [filtered, query]);

  function displayValue() {
    if (open) return query;
    return selected?.label ?? query;
  }

  function choose(opt: ComboOption) {
    onSelect(opt);
    setQuery(opt.label);
    setOpen(false);
    setAdding(false);
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (disabled) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setHighlight((h) => Math.min(h + 1, Math.max(0, filtered.length - 1)));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setOpen(true);
      setHighlight((h) => Math.max(h - 1, 0));
    } else if (e.key === "Enter") {
      if (open && filtered[highlight]) {
        e.preventDefault();
        choose(filtered[highlight]!);
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      setAdding(false);
    }
  }

  async function saveNew() {
    if (!onAddNew) return;
    const p = primary.trim();
    if (!p) {
      setAddError(addNewFields?.primary ? `${addNewFields.primary} is required` : "Required");
      return;
    }
    setSaving(true);
    setAddError("");
    try {
      const opt = await onAddNew({
        primary: p,
        secondary: secondary.trim() || undefined,
      });
      choose(opt);
      setPrimary("");
      setSecondary("");
    } catch (err) {
      setAddError(err instanceof Error ? err.message : "Could not save");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className={`searchable-combo${disabled ? " is-disabled" : ""}`} ref={rootRef}>
      <label className="searchable-combo-label">{label}</label>
      <div className="searchable-combo-row">
        <Input
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={open && filtered[highlight] ? `${listId}-${highlight}` : undefined}
          value={displayValue()}
          disabled={disabled}
          placeholder={placeholder}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            if (value) onSelect({ value: "", label: "" });
          }}
          onFocus={() => {
            if (!disabled) {
              setOpen(true);
              if (selected && !query) setQuery(selected.label);
            }
          }}
          onKeyDown={onKeyDown}
          autoComplete="off"
        />
        {onAddNew ? (
          <button
            type="button"
            className="searchable-combo-add"
            aria-label="Add new"
            aria-expanded={adding}
            disabled={disabled}
            onClick={() => {
              setAdding((v) => !v);
              setOpen(false);
              setAddError("");
              // Prefill Add New from whatever was typed in the search box.
              if (query.trim() && !primary) {
                const digits = query.replace(/\D/g, "");
                if (digits.length >= 9 && addNewFields?.secondary) {
                  setSecondary(query.trim());
                } else {
                  setPrimary(query.trim());
                }
              }
            }}
          >
            <Icon d="M12 5v14" extra={<path d="M5 12h14" />} size={16} />
          </button>
        ) : null}
      </div>

      <div className="searchable-combo-panel-slot" aria-hidden={!open && !adding}>
        {open && !adding ? (
          <ul id={listId} className="searchable-combo-list" role="listbox">
            {loading ? (
              <li className="searchable-combo-empty" role="presentation">
                Searching…
              </li>
            ) : filtered.length === 0 ? (
              <li className="searchable-combo-empty" role="presentation">
                No matches{onAddNew ? " — tap + to add" : ""}
              </li>
            ) : (
              filtered.map((opt, i) => (
                <li key={opt.value || `opt-${i}`} role="option" aria-selected={i === highlight} id={`${listId}-${i}`}>
                  <button
                    type="button"
                    className={`searchable-combo-option${i === highlight ? " is-active" : ""}${opt.value === value ? " is-selected" : ""}`}
                    onMouseEnter={() => setHighlight(i)}
                    onClick={() => choose(opt)}
                  >
                    <span>{opt.label}</span>
                    {opt.sublabel ? <span className="muted">{opt.sublabel}</span> : null}
                  </button>
                </li>
              ))
            )}
          </ul>
        ) : null}

        {adding && onAddNew ? (
          <div className="searchable-combo-add-panel">
            <label>
              {addNewFields?.primary ?? "Name"}
              <Input
                value={primary}
                onChange={(e) => setPrimary(e.target.value)}
                placeholder={addNewFields?.primary ?? "Name"}
                autoFocus
              />
            </label>
            {addNewFields?.secondary ? (
              <label>
                {addNewFields.secondary}
                <Input
                  value={secondary}
                  onChange={(e) => setSecondary(e.target.value)}
                  placeholder={addNewFields.secondary}
                />
              </label>
            ) : null}
            {addError ? <p className="error">{addError}</p> : null}
            <div className="chip-row">
              <Button type="button" disabled={saving} onClick={() => void saveNew()}>
                {saving ? "Saving…" : "Save"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setAdding(false);
                  setAddError("");
                }}
              >
                Cancel
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
