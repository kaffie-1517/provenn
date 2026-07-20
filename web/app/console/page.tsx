"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import styles from "./console.module.css";

interface Partner {
  id: string;
  name: string;
  created_at: string;
  invoice_count_30d: number;
}

interface Company {
  id: string;
  name: string;
  plan: string;
  created_at: string;
  verification_count_30d: number;
}

export default function PlatformConsole() {
  const router = useRouter();
  const [token, setToken] = useState<string | null>(null);
  const [partners, setPartners] = useState<Partner[]>([]);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeView, setActiveView] = useState<"partners" | "companies">(
    "partners"
  );

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

  const fetchData = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError("");
    try {
      const [pRes, cRes] = await Promise.all([
        fetch(`${apiBase}/api/v1/admin/partners`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
        fetch(`${apiBase}/api/v1/admin/companies`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
      ]);

      if (pRes.status === 403 || cRes.status === 403) {
        setError("Access denied. Platform admin role required.");
        setLoading(false);
        return;
      }

      const pData = await pRes.json();
      const cData = await cRes.json();
      setPartners(pData.partners || []);
      setCompanies(cData.companies || []);
    } catch {
      setError("Failed to load platform data");
    } finally {
      setLoading(false);
    }
  }, [token, apiBase]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  if (!token) return null;

  return (
    <div className={styles.container}>
      <div className={styles.wrapper}>
        {/* ── Header ─────────────────────────────────────────── */}
        <div className={styles.header}>
          <div className={styles.headerBadge}>Portal 2</div>
          <h1 className={styles.title}>Platform Console</h1>
          <p className={styles.subtitle}>
            Cross-tenant usage overview — partners & companies
          </p>
        </div>

        {error && <div className={styles.error}>{error}</div>}

        {/* ── Summary Cards ──────────────────────────────────── */}
        <div className={styles.summaryRow}>
          <div className={styles.summaryCard}>
            <span className={styles.summaryValue}>{partners.length}</span>
            <span className={styles.summaryLabel}>Partners</span>
          </div>
          <div className={styles.summaryCard}>
            <span className={styles.summaryValue}>{companies.length}</span>
            <span className={styles.summaryLabel}>Companies</span>
          </div>
          <div className={styles.summaryCard}>
            <span className={styles.summaryValue}>
              {partners.reduce((s, p) => s + p.invoice_count_30d, 0)}
            </span>
            <span className={styles.summaryLabel}>Invoices (30d)</span>
          </div>
          <div className={styles.summaryCard}>
            <span className={styles.summaryValue}>
              {companies.reduce((s, c) => s + c.verification_count_30d, 0)}
            </span>
            <span className={styles.summaryLabel}>Verifications (30d)</span>
          </div>
        </div>

        {/* ── View Tabs ──────────────────────────────────────── */}
        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${activeView === "partners" ? styles.activeTab : ""}`}
            onClick={() => setActiveView("partners")}
          >
            Partners
          </button>
          <button
            className={`${styles.tab} ${activeView === "companies" ? styles.activeTab : ""}`}
            onClick={() => setActiveView("companies")}
          >
            Companies
          </button>
        </div>

        {/* ── Content ────────────────────────────────────────── */}
        <div className={styles.card}>
          {loading ? (
            <div className={styles.spinner} />
          ) : activeView === "partners" ? (
            partners.length === 0 ? (
              <p className={styles.emptyText}>No partners found.</p>
            ) : (
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Partner</th>
                    <th>Created</th>
                    <th className={styles.numCol}>Invoices (30d)</th>
                  </tr>
                </thead>
                <tbody>
                  {partners.map((p) => (
                    <tr key={p.id}>
                      <td className={styles.nameCell}>{p.name}</td>
                      <td className={styles.dateCell}>
                        {new Date(p.created_at).toLocaleDateString()}
                      </td>
                      <td className={styles.numCell}>
                        <span className={styles.countBadge}>
                          {p.invoice_count_30d}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )
          ) : companies.length === 0 ? (
            <p className={styles.emptyText}>No companies found.</p>
          ) : (
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Company</th>
                  <th>Plan</th>
                  <th>Created</th>
                  <th className={styles.numCol}>Verifications (30d)</th>
                </tr>
              </thead>
              <tbody>
                {companies.map((c) => (
                  <tr key={c.id}>
                    <td className={styles.nameCell}>{c.name}</td>
                    <td>
                      <span className={styles.planBadge}>{c.plan}</span>
                    </td>
                    <td className={styles.dateCell}>
                      {new Date(c.created_at).toLocaleDateString()}
                    </td>
                    <td className={styles.numCell}>
                      <span className={styles.countBadge}>
                        {c.verification_count_30d}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
