import { useEffect, useState, type FormEvent } from "react";
import { sendCustomSMS } from "../lib/api";
import { Button, Input, Textarea } from "./ui";

const GSM_SEGMENT = 160;
const UCS2_SEGMENT = 70;

function isGsm7(text: string): boolean {
  // GSM-7 printable + a few extended symbols. Anything else uses UCS-2 (70/seg).
  for (const ch of text) {
    const code = ch.codePointAt(0) ?? 0;
    if (code > 0x7f && !"£¥èéùìòÇØøÅåΔ_ΦΓΛΩΠΨΣΘΞ^{}\\[~]|€".includes(ch)) {
      return false;
    }
  }
  return true;
}

function smsStats(text: string): { chars: number; segments: number; perSegment: number } {
  const chars = [...text].length;
  const perSegment = isGsm7(text) ? GSM_SEGMENT : UCS2_SEGMENT;
  const segments = chars === 0 ? 0 : Math.ceil(chars / perSegment);
  return { chars, segments, perSegment };
}

export type SendSmsModalProps = {
  open: boolean;
  onClose: () => void;
  initialPhone?: string;
  customerId?: string;
  repairJobId?: string;
  title?: string;
};

export function SendSmsModal({
  open,
  onClose,
  initialPhone = "",
  customerId,
  repairJobId,
  title = "Send SMS",
}: SendSmsModalProps) {
  const [phone, setPhone] = useState(initialPhone);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState("");

  useEffect(() => {
    if (!open) return;
    setPhone(initialPhone);
    setMessage("");
    setError("");
    setDone("");
    setBusy(false);
  }, [open, initialPhone]);

  if (!open) return null;

  const stats = smsStats(message);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setDone("");
    try {
      await sendCustomSMS({
        phone: phone.trim(),
        message: message.trim(),
        customer_id: customerId,
        repair_job_id: repairJobId,
      });
      setDone(`Queued to ${phone.trim()}`);
      setMessage("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Send failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="cmdk-backdrop"
      role="presentation"
      onClick={() => {
        if (!busy) onClose();
      }}
    >
      <div
        className="cmdk-panel estimate-modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="panel-head" style={{ padding: "1rem 1.25rem 0" }}>
          <h2 style={{ margin: 0 }}>{title}</h2>
        </div>
        <form className="form-grid" style={{ padding: "1rem 1.25rem 1.25rem" }} onSubmit={onSubmit}>
          <label>
            Phone
            <Input
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              inputMode="tel"
              placeholder="07… or 254…"
              required
              autoFocus={!initialPhone}
            />
          </label>
          <label>
            Message
            <Textarea
              rows={5}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="Type the SMS the customer will receive…"
              required
              autoFocus={Boolean(initialPhone)}
            />
            <span className="hint">
              {stats.chars === 0
                ? `0/${stats.perSegment}`
                : `${stats.chars} characters · ${stats.segments} segment${stats.segments === 1 ? "" : "s"} (${stats.perSegment}/segment)`}
            </span>
          </label>
          {error ? (
            <p className="form-error" role="alert">
              {error}
            </p>
          ) : null}
          {done ? (
            <p className="hint" role="status">
              {done}
            </p>
          ) : null}
          <div className="btn-row">
            <Button type="submit" disabled={busy || !phone.trim() || !message.trim()}>
              {busy ? "Sending…" : "Send SMS"}
            </Button>
            <Button type="button" variant="secondary" disabled={busy} onClick={onClose}>
              {done ? "Close" : "Cancel"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
