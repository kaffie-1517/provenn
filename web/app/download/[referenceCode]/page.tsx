"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import styles from "./download.module.css";

interface InvoiceInfo {
  invoice_id: string;
  reference_code: string;
  status: string;
  ready: boolean;
  amount_cents: number;
  currency: string;
  vendor_name: string;
}

export default function DownloadPage() {
  const params = useParams();
  const referenceCode = params.referenceCode as string;

  const [invoice, setInvoice] = useState<InvoiceInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [downloading, setDownloading] = useState(false);
  const [downloaded, setDownloaded] = useState(false);

  const apiBase =
    process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

  useEffect(() => {
    async function fetchStatus() {
      try {
        const res = await fetch(
          `${apiBase}/api/v1/invoices/${referenceCode}`
        );
        if (!res.ok) {
          setError("Invoice not found");
          return;
        }
        const data = await res.json();
        setInvoice(data);
      } catch {
        setError("Failed to fetch invoice details");
      } finally {
        setLoading(false);
      }
    }
    fetchStatus();
  }, [referenceCode, apiBase]);

  function formatAmount(cents: number, currency: string) {
    const amount = (cents / 100).toFixed(2);
    const symbols: Record<string, string> = {
      INR: "₹",
      USD: "$",
      EUR: "€",
    };
    return `${symbols[currency] || currency} ${amount}`;
  }

  async function handleConfirmDownload() {
    setDownloading(true);
    try {
      const res = await fetch(
        `${apiBase}/api/v1/invoices/${referenceCode}/download`
      );
      if (res.status === 202) {
        setError("Invoice is still processing. Please try again shortly.");
        setDownloading(false);
        return;
      }
      if (!res.ok) {
        setError("Download failed");
        setDownloading(false);
        return;
      }

      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `invoice-${referenceCode}.pdf`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      setDownloaded(true);
    } catch {
      setError("Download failed");
    } finally {
      setDownloading(false);
    }
  }

  if (loading) {
    return (
      <div className={styles.container}>
        <div className={styles.card}>
          <div className={styles.spinner} />
          <p className={styles.loadingText}>Loading invoice details…</p>
        </div>
      </div>
    );
  }

  if (error && !invoice) {
    return (
      <div className={styles.container}>
        <div className={styles.card}>
          <div className={styles.errorIcon}>✕</div>
          <h1 className={styles.title}>Invoice Not Found</h1>
          <p className={styles.subtitle}>
            The reference code <code>{referenceCode}</code> does not match
            any invoice in our system.
          </p>
        </div>
      </div>
    );
  }

  if (!invoice) return null;

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        {!downloaded ? (
          <>
            <div className={styles.iconWrap}>
              <svg
                className={styles.icon}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="7 10 12 15 17 10" />
                <line x1="12" y1="15" x2="12" y2="3" />
              </svg>
            </div>

            <h1 className={styles.title}>Download Invoice</h1>
            <p className={styles.subtitle}>
              Please confirm you want to download this invoice.
              {!invoice.ready && " The invoice is still being processed."}
            </p>

            {error && <div className={styles.error}>{error}</div>}

            <div className={styles.details}>
              <div className={styles.detailRow}>
                <span className={styles.detailLabel}>Reference</span>
                <span className={styles.detailValue}>
                  {invoice.reference_code}
                </span>
              </div>
              <div className={styles.detailRow}>
                <span className={styles.detailLabel}>Vendor</span>
                <span className={styles.detailValue}>
                  {invoice.vendor_name}
                </span>
              </div>
              <div className={styles.detailRow}>
                <span className={styles.detailLabel}>Amount</span>
                <span className={styles.detailValue}>
                  {formatAmount(invoice.amount_cents, invoice.currency)}
                </span>
              </div>
              <div className={styles.detailRow}>
                <span className={styles.detailLabel}>Status</span>
                <span
                  className={`${styles.statusBadge} ${
                    invoice.ready ? styles.ready : styles.processing
                  }`}
                >
                  {invoice.ready ? "Ready" : "Processing"}
                </span>
              </div>
            </div>

            <button
              className={styles.downloadButton}
              onClick={handleConfirmDownload}
              disabled={!invoice.ready || downloading}
            >
              {downloading
                ? "Downloading…"
                : invoice.ready
                  ? "Confirm & Download"
                  : "Not Ready Yet"}
            </button>

            <p className={styles.notice}>
              By downloading, you acknowledge receipt of this invoice.
              A billing record will be created.
            </p>
          </>
        ) : (
          <>
            <div className={styles.successIcon}>✓</div>
            <h1 className={styles.title}>Download Complete</h1>
            <p className={styles.subtitle}>
              Your invoice <strong>{invoice.reference_code}</strong> has
              been downloaded successfully.
            </p>
            <button
              className={styles.downloadButton}
              onClick={handleConfirmDownload}
            >
              Download Again
            </button>
          </>
        )}
      </div>
    </div>
  );
}
