import { useEffect, useState } from "react";

function remaining(target: number) {
  const diff = Math.max(0, target - Date.now());
  const days = Math.floor(diff / 86_400_000);
  const hours = Math.floor((diff % 86_400_000) / 3_600_000);
  const minutes = Math.floor((diff % 3_600_000) / 60_000);
  const seconds = Math.floor((diff % 60_000) / 1000);
  return { days, hours, minutes, seconds, done: diff <= 0 };
}

/** Client-side display only — the deal is already enforced server-side at checkout. */
export function Countdown({ endsAt }: { endsAt: string }) {
  const target = new Date(endsAt).getTime();
  const [now, setNow] = useState(() => remaining(target));

  useEffect(() => {
    const t = window.setInterval(() => setNow(remaining(target)), 1000);
    return () => window.clearInterval(t);
  }, [target]);

  if (now.done) return <span className="countdown countdown-ended">Deal ended</span>;

  return (
    <span className="countdown">
      {now.days > 0 ? `${now.days}d ` : ""}
      {String(now.hours).padStart(2, "0")}:{String(now.minutes).padStart(2, "0")}:{String(now.seconds).padStart(2, "0")}
    </span>
  );
}
