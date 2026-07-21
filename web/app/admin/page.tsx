"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import styles from "./admin.module.css";

interface Verification {
  id: string;
  invoice_id: string | null;
  company_id: string;
  submitted_by: string;
  submitted_at: string;
  submitted_hash: string;
  matched_version_id: string | null;
  result: string;
  approval_status: string;
  approved_by: string | null;
  approved_at: string | null;
  employee_email?: string;
  vendor_name?: string;
  amount_cents?: number;
  currency?: string;
}

type FilterTab = "all" | "pending" | "approved" | "rejected";

export default function AdminPage() {
  const router = useRouter();
  const [token, setToken] = useState<string | null>(null);
  const [verifications, setVerifications] = useState<Verification[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<FilterTab>("pending");
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [exporting, setExporting] = useState(false);

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
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      if (activeTab !== "all") {
        params.set("approval_status", activeTab);
      }
      const url = `${apiBase}/api/v1/verifications?${params}`;
      const res = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.status === 403) {
        setError("Access denied. You must be a company admin.");
        return;
      }
      if (!res.ok) {
        setError("Failed to load verifications");
        return;
      }
      const data = await res.json();
      setVerifications(data.verifications || []);
    } catch {
      setError("Failed to load verifications");
    } finally {
      setLoading(false);
    }
  }, [token, activeTab, apiBase]);

  useEffect(() => {
    fetchVerifications();
  }, [fetchVerifications]);

  async function handleDecision(verifId: string, decision: "approved" | "rejected") {
    setActionLoading(verifId);
    try {
      const res = await fetch(
        `${apiBase}/api/v1/verifications/${verifId}/approve`,
        {
          method: "PATCH",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ decision }),
        }
      );

      if (!res.ok) {
        const data = await res.json();
        setError(data.error || "Action failed");
        return;
      }

      // Refresh the list.
      fetchVerifications();
    } catch {
      setError("Action failed");
    } finally {
      setActionLoading(null);
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

  async function handleExport() {
    if (!token) return;
    setExporting(true);
    try {
      const res = await fetch(`${apiBase}/api/v1/verifications/export`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        setError("Export failed");
        return;
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "approved_verifications.xlsx";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch {
      setError("Export failed");
    } finally {
      setExporting(false);
    }
  }

  if (!token) return null;

  return (
    <div className={styles.container}>
      <div className={styles.wrapper}>
        <div className={styles.header}>
          <h1 className={styles.title}>Verification Dashboard</h1>
          <p className={styles.subtitle}>
            Review and approve employee invoice submissions
          </p>
          <button
            className={styles.exportButton}
            onClick={handleExport}
            disabled={exporting}
          >
            {exporting ? "Exporting…" : "⬇ Export Approved (.xlsx)"}
          </button>
        </div>

        {error && <div className={styles.error}>{error}</div>}

        {/* ── Filter Tabs ──────────────────────────────────────── */}
        <div className={styles.tabs}>
          {(["pending", "approved", "rejected", "all"] as FilterTab[]).map(
            (tab) => (
              <button
                key={tab}
                className={`${styles.tab} ${activeTab === tab ? styles.activeTab : ""}`}
                onClick={() => setActiveTab(tab)}
              >
                {tab.charAt(0).toUpperCase() + tab.slice(1)}
              </button>
            )
          )}
        </div>

        {/* ── Verifications List ───────────────────────────────── */}
        <div className={styles.card}>
          {loading ? (
            <div className={styles.spinner} />
          ) : verifications.length === 0 ? (
            <p className={styles.emptyText}>
              No {activeTab === "all" ? "" : activeTab} verifications found.
            </p>
          ) : (
            <div className={styles.list}>
              {verifications.map((v) => (
                <div key={v.id} className={styles.verifCard}>
                  <div className={styles.verifTop}>
                    <div className={styles.verifInfo}>
                      <div className={styles.verifRow}>
                        <span className={styles.label}>Result</span>
                        {resultBadge(v.result)}
                      </div>
                      <div className={styles.verifRow}>
                        <span className={styles.label}>Status</span>
                        {approvalBadge(v.approval_status)}
                      </div>
                      <div className={styles.verifRow}>
                        <span className={styles.label}>Hash</span>
                        <code className={styles.hash}>
                          {v.submitted_hash?.slice(0, 16)}…
                        </code>
                      </div>
                      <div className={styles.verifRow}>
                        <span className={styles.label}>Submitted</span>
                        <span className={styles.date}>
                          {new Date(v.submitted_at).toLocaleString()}
                        </span>
                      </div>
                      <div className={styles.verifRow}>
                        <span className={styles.label}>Submitted By</span>
                        <span className={styles.value}>{v.employee_email || "Unknown"}</span>
                      </div>
                      {v.vendor_name ? (
                        <>
                          <div className={styles.verifRow}>
                            <span className={styles.label}>Vendor</span>
                            <span className={styles.value}>{v.vendor_name}</span>
                          </div>
                          <div className={styles.verifRow}>
                            <span className={styles.label}>Amount</span>
                            <span className={styles.value}>
                              {v.amount_cents ? (v.amount_cents / 100).toFixed(2) : "0.00"} {v.currency}
                            </span>
                          </div>
                        </>
                      ) : (
                        <div className={styles.verifRow}>
                          <span className={styles.label}>Invoice Details</span>
                          <span className={styles.value} style={{ opacity: 0.7, fontStyle: "italic" }}>
                            Unknown (Not Found)
                          </span>
                        </div>
                      )}
                    </div>
                  </div>

                  {v.approval_status === "pending" && (
                    <div className={styles.actions}>
                      <button
                        className={`${styles.actionBtn} ${styles.approveBtn}`}
                        onClick={() => handleDecision(v.id, "approved")}
                        disabled={actionLoading === v.id}
                      >
                        {actionLoading === v.id
                          ? "…"
                          : "✓ Approve"}
                      </button>
                      <button
                        className={`${styles.actionBtn} ${styles.rejectBtn}`}
                        onClick={() => handleDecision(v.id, "rejected")}
                        disabled={actionLoading === v.id}
                      >
                        {actionLoading === v.id
                          ? "…"
                          : "✕ Reject"}
                      </button>
                    </div>
                  )}

                  {v.approved_at && (
                    <div className={styles.approvedInfo}>
                      Decided on {new Date(v.approved_at).toLocaleString()}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
