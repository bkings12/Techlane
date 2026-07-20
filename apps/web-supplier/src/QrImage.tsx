import { useEffect, useState } from "react";
import QRCode from "qrcode";

export function QrImage({ payload, size = 220 }: { payload: string; size?: number }) {
  const [src, setSrc] = useState("");

  useEffect(() => {
    if (!payload) {
      setSrc("");
      return;
    }
    void QRCode.toDataURL(payload, { width: size, margin: 1 })
      .then(setSrc)
      .catch(() => setSrc(""));
  }, [payload, size]);

  if (!src) return <div className="qr-box">Generating QR…</div>;
  return <img className="qr-image" src={src} alt="Collection QR code" width={size} height={size} />;
}
