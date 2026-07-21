"use client";

import { useState, useEffect, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import styles from "./provider.module.css";

interface InvoiceResult {
  invoice_id: string;
  reference_code: string;
  status: string;
}

interface InvoiceStatus {
  invoice_id: string;
  reference_code: string;
  status: string;
  ready: boolean;
  amount_cents: number;
  currency: string;
  vendor_name: string;
}

export default function ProviderPage() {
  const router = useRouter();
  const { token, isAuthenticated, user } = useAuth();

  const [vendorName, setVendorName] = useState("");
  const [amount, setAmount] = useState("");
  const [currency, setCurrency] = useState("INR");
  const [invoiceDate, setInvoiceDate] = useState(
    new Date().toISOString().split("T")[0]
  );
  const [pdfFile, setPdfFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  // After creation — polling state
  const [result, setResult] = useState<InvoiceResult | null>(null);
  const [status, setStatus] = useState<InvoiceStatus | null>(null);
  const [hasDownloaded, setHasDownloaded] = useState(false);

  // Redirect if not authenticated or not a provider
  useEffect(() => {
    if (!isAuthenticated) {
      router.push("/login");
    }
  }, [isAuthenticated, router]);

  // Poll for invoice status after creation
  useEffect(() => {
    if (!result) return;
    let cancelled = false;

    const poll = async () => {
      while (!cancelled) {
        try {
          const s = await api<InvoiceStatus>(
            `/api/v1/invoices/${result.reference_code}`
          );
          setStatus(s);
          if (s.ready) return; // done
        } catch {
          // ignore poll errors
        }
        await new Promise((r) => setTimeout(r, 1500));
      }
    };
    poll();

    return () => {
      cancelled = true;
    };
  }, [result]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    setResult(null);
    setStatus(null);
    setHasDownloaded(false);

    if (!pdfFile) {
      setError("Please select a PDF file");
      setLoading(false);
      return;
    }

    try {
      const amountInCents = Math.round(parseFloat(amount) * 100).toString();

      const formData = new FormData();
      formData.append("pdf", pdfFile);
      formData.append("amount_cents", amountInCents);
      formData.append("currency", currency);
      formData.append("vendor_name", vendorName);
      formData.append("invoice_date", invoiceDate);

      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/v1/invoices`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
          },
          body: formData,
        }
      );

      const data = await res.json();
      if (!res.ok) throw new ApiError(res.status, data.error || "Failed");

      setResult(data);
    } catch (err) {
      if (err instanceof ApiError) setError(err.message);
      else setError("Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  if (!isAuthenticated) return null;

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.header}>
          <h1 className={styles.title}>Issue Invoice</h1>
          <p className={styles.subtitle}>
            Logged in as <strong>{user?.email}</strong> ({user?.role})
          </p>
        </div>

        {!result ? (
          <form onSubmit={handleSubmit} className={styles.form}>
            {error && <div className={styles.error}>{error}</div>}

            <div className={styles.row}>
              <div className={styles.field}>
                <label htmlFor="vendorName" className={styles.label}>
                  Vendor Name
                </label>
                <input
                  id="vendorName"
                  type="text"
                  value={vendorName}
                  onChange={(e) => setVendorName(e.target.value)}
                  className={styles.input}
                  placeholder="Demo Cabs"
                  required
                />
              </div>
              <div className={styles.field}>
                <label htmlFor="invoiceDate" className={styles.label}>
                  Invoice Date
                </label>
                <input
                  id="invoiceDate"
                  type="date"
                  value={invoiceDate}
                  onChange={(e) => setInvoiceDate(e.target.value)}
                  className={styles.input}
                  required
                />
              </div>
            </div>

            <div className={styles.row}>
              <div className={styles.field}>
                <label htmlFor="amount" className={styles.label}>
                  Amount
                </label>
                <input
                  id="amount"
                  type="number"
                  step="0.01"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  className={styles.input}
                  placeholder="1500.00"
                  min="0.01"
                  required
                />
              </div>
              <div className={styles.field}>
                <label htmlFor="currency" className={styles.label}>
                  Currency
                </label>
                <select
                  id="currency"
                  value={currency}
                  onChange={(e) => setCurrency(e.target.value)}
                  className={styles.input}
                >
                  <option value="INR">INR</option>
                  <option value="USD">USD</option>
                  <option value="EUR">EUR</option>
                </select>
              </div>
            </div>

            <div className={styles.field}>
              <label htmlFor="pdf" className={styles.label}>
                Invoice PDF
              </label>
              <input
                id="pdf"
                type="file"
                accept=".pdf"
                onChange={(e) => setPdfFile(e.target.files?.[0] || null)}
                className={styles.fileInput}
                required
              />
            </div>

            <button type="submit" disabled={loading} className={styles.button}>
              {loading ? "Issuing…" : "Issue Invoice"}
            </button>
          </form>
        ) : (
          <div className={styles.result}>
            <div className={styles.resultHeader}>
              <span className={styles.checkIcon}>✓</span>
              <h2>Invoice Created</h2>
            </div>

            <div className={styles.resultGrid}>
              <div className={styles.resultItem}>
                <span className={styles.resultLabel}>Reference Code</span>
                <span className={styles.refCode}>
                  {result.reference_code}
                </span>
              </div>
              <div className={styles.resultItem}>
                <span className={styles.resultLabel}>Invoice ID</span>
                <span className={styles.resultValue}>
                  {result.invoice_id.slice(0, 8)}…
                </span>
              </div>
              <div className={styles.resultItem}>
                <span className={styles.resultLabel}>Status</span>
                <span
                  className={`${styles.statusBadge} ${
                    status?.ready ? styles.ready : styles.processing
                  }`}
                >
                  {status?.ready ? "Ready" : "Processing…"}
                </span>
              </div>
            </div>

            {!status?.ready && (
              <p className={styles.pollingNote}>
                Polling for status updates…
              </p>
            )}

            {status?.ready && (
              <div className={styles.row} style={{ marginTop: "1rem" }}>
                <button
                  className={styles.button}
                  disabled={hasDownloaded}
                  onClick={() => {
                    setHasDownloaded(true);
                    window.open(
                      `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/v1/invoices/${result.reference_code}/download`,
                      "_blank"
                    );
                  }}
                >
                  {hasDownloaded ? "Downloaded" : "Download PDF"}
                </button>
              </div>
            )}

            <button
              className={styles.buttonSecondary}
              onClick={() => {
                setResult(null);
                setStatus(null);
              }}
            >
              Issue Another
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
