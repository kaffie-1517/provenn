"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import styles from "./employee.module.css";

interface Verification {
  verification_id: string;
  invoice_id: string | null;
  reference_code: string;
  result: string;
  submitted_hash: string;
  matched_version_id: string | null;
  approval_status: string;
  submitted_at: string;
}

interface ListResponse {
  verifications: Verification[];
  total: number;
}

export default function EmployeePage() {
  const router = useRouter();
  const [token, setToken] = useState<string | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<Verification | null>(null);
  const [error, setError] = useState("");
  const [verifications, setVerifications] = useState<Verification[]>([]);
  const [loadingList, setLoadingList] = useState(false);

  const apiBase =
    process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

  useEffect(() => {
    const stored = localStorage.getItem("provenn_token");
    if (!stored) {
      router.push("/login");
      return;
    }
    setToken(stored);
  }, [router]);

  const fetchVerifications = useCallback(async () => {
    if (!token) return;
    setLoadingList(true);
    try {
      const res = await fetch(`${apiBase}/api/v1/verifications`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        const data: ListResponse = await res.json();
        setVerifications(data.verifications || []);
      }
    } catch {
      // Silently fail — list is secondary
    } finally {
      setLoadingList(false);
    }
  }, [token, apiBase]);

  useEffect(() => {
    fetchVerifications();
  }, [fetchVerifications]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file || !token) return;

    setSubmitting(true);
    setError("");
    setResult(null);

    try {
      const fd = new FormData();
      fd.append("pdf", file);

      const res = await fetch(`${apiBase}/api/v1/verifications`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Submission failed");
        return;
      }

      setResult(data);
      setFile(null);
      // Refresh the list
      fetchVerifications();
    } catch {
      setError("Submission failed");
    } finally {
      setSubmitting(false);
    }
  }

  function resultBadge(r: string) {
    const cls =
      r === "match"
        ? styles.match
        : r === "mismatch"
          ? styles.mismatch
          : styles.notFound;
    return <span className={`${styles.badge} ${cls}`}>{r}</span>;
  }

  function approvalBadge(s: string) {
    const cls =
      s === "approved"
        ? styles.approved
        : s === "rejected"
          ? styles.rejected
          : styles.pending;
    return <span className={`${styles.badge} ${cls}`}>{s}</span>;
  }

  if (!token) return null;

  return (
    <div className={styles.container}>
      <div className={styles.wrapper}>
        {/* ── Submit Section ───────────────────────────────────────── */}
        <div className={styles.card}>
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
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
              <line x1="12" y1="18" x2="12" y2="12" />
              <line x1="9" y1="15" x2="15" y2="15" />
            </svg>
          </div>
          <h1 className={styles.title}>Submit Invoice for Verification</h1>
          <p className={styles.subtitle}>
            Upload a PDF invoice. The system will extract the reference code,
            compare the document hash, and return a verification result.
          </p>

          {error && <div className={styles.error}>{error}</div>}

          {result && (
            <div
              className={`${styles.resultCard} ${
                result.result === "match"
                  ? styles.resultMatch
                  : result.result === "mismatch"
                    ? styles.resultMismatch
                    : styles.resultNotFound
              }`}
            >
              <div className={styles.resultTitle}>
                {result.result === "match"
                  ? "✓ Verified — Document is authentic"
                  : result.result === "mismatch"
                    ? "✕ Mismatch — Document has been modified"
                    : "? Not Found — No matching invoice"}
              </div>
              {result.reference_code && (
                <div className={styles.resultDetail}>
                  Reference: <strong>{result.reference_code}</strong>
                </div>
              )}
              <div className={styles.resultDetail}>
                Hash: <code>{result.submitted_hash?.slice(0, 16)}…</code>
              </div>
              <div className={styles.resultDetail}>
                Approval: {approvalBadge(result.approval_status)}
              </div>
            </div>
          )}

          <form onSubmit={handleSubmit} className={styles.form}>
            <label className={styles.fileLabel}>
              <input
                type="file"
                accept=".pdf"
                className={styles.fileInput}
                onChange={(e) => setFile(e.target.files?.[0] || null)}
              />
              <span className={styles.fileLabelText}>
                {file ? file.name : "Choose a PDF file…"}
              </span>
            </label>

            <button
              type="submit"
              className={styles.submitButton}
              disabled={!file || submitting}
            >
              {submitting ? "Verifying…" : "Submit for Verification"}
            </button>
          </form>
        </div>

        {/* ── History Section ──────────────────────────────────────── */}
        <div className={styles.card}>
          <h2 className={styles.sectionTitle}>My Submissions</h2>
          {loadingList ? (
            <div className={styles.spinner} />
          ) : verifications.length === 0 ? (
            <p className={styles.emptyText}>
              No submissions yet. Upload an invoice above.
            </p>
          ) : (
            <div className={styles.table}>
              <div className={styles.tableHeader}>
                <span>Reference</span>
                <span>Result</span>
                <span>Approval</span>
                <span>Submitted</span>
              </div>
              {verifications.map((v) => (
                <div key={v.verification_id} className={styles.tableRow}>
                  <span className={styles.refCode}>
                    {v.reference_code || "—"}
                  </span>
                  <span>{resultBadge(v.result)}</span>
                  <span>{approvalBadge(v.approval_status)}</span>
                  <span className={styles.date}>
                    {new Date(v.submitted_at).toLocaleDateString()}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
