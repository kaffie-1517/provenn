"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { api, ApiError } from "@/lib/api";
import styles from "../login/login.module.css";

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("employee");
  const [companyId, setCompanyId] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const body: Record<string, string | undefined> = {
        email,
        password,
        role,
      };
      if (role !== "platform_admin" && companyId) {
        body.company_id = companyId;
      }

      await api("/api/v1/auth/register", {
        method: "POST",
        body,
      });

      // Redirect to login on success.
      router.push("/login");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Something went wrong");
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.header}>
          <h1 className={styles.logo}>ProveNN</h1>
          <p className={styles.subtitle}>Create your account</p>
        </div>

        <form onSubmit={handleSubmit} className={styles.form}>
          {error && <div className={styles.error}>{error}</div>}

          <div className={styles.field}>
            <label htmlFor="email" className={styles.label}>
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className={styles.input}
              placeholder="you@company.com"
              required
              autoComplete="email"
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="password" className={styles.label}>
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={styles.input}
              placeholder="••••••••"
              required
              minLength={6}
              autoComplete="new-password"
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="role" className={styles.label}>
              Role
            </label>
            <select
              id="role"
              value={role}
              onChange={(e) => setRole(e.target.value)}
              className={styles.input}
            >
              <option value="employee">Employee</option>
              <option value="company_admin">Company Admin</option>
              <option value="provider">Provider</option>
              <option value="platform_admin">Platform Admin</option>
            </select>
          </div>

          {role !== "platform_admin" && (
            <div className={styles.field}>
              <label htmlFor="companyId" className={styles.label}>
                Company ID
              </label>
              <input
                id="companyId"
                type="text"
                value={companyId}
                onChange={(e) => setCompanyId(e.target.value)}
                className={styles.input}
                placeholder="UUID of your company"
                required
              />
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className={styles.button}
          >
            {loading ? "Creating account…" : "Create account"}
          </button>
        </form>

        <p className={styles.footer}>
          Already have an account?{" "}
          <Link href="/login" className={styles.link}>
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
