"use client";

import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import styles from "./page.module.css";

interface VerificationResult {
  verification_id: string;
  result: "match" | "mismatch";
  message: string;
}

export default function EmployeePortal() {
  const router = useRouter();
  const [token, setToken] = useState<string | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<VerificationResult | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const apiBase = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

  useEffect(() => {
    const stored = localStorage.getItem("provenn_token");
    if (!stored) {
      router.push("/login");
      return;
    }
    setToken(stored);
  }, [router]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      setFile(e.target.files[0]);
      setError("");
      setResult(null);
    }
  };

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      setFile(e.dataTransfer.files[0]);
      setError("");
      setResult(null);
    }
  };

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
  };

  const handleSubmit = async () => {
    if (!file || !token) return;
    
    setLoading(true);
    setError("");
    setResult(null);

    const formData = new FormData();
    formData.append("pdf", file);

    try {
      const res = await fetch(`${apiBase}/api/v1/verifications`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Failed to submit verification");
      }

      setResult(data as VerificationResult);
    } catch (err: any) {
      setError(err.message || "An unexpected error occurred");
    } finally {
      setLoading(false);
    }
  };

  if (!token) return null; // Or a loading spinner while redirecting

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Employee Portal</h1>
        <p className={styles.subtitle}>Submit invoices for verification</p>
      </div>

      <div className={styles.card}>
        <div 
          className={styles.dropzone}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onClick={() => fileInputRef.current?.click()}
        >
          <p>Drag and drop your invoice PDF here</p>
          <p>or</p>
          <button className={styles.selectButton}>Select File</button>
          <input 
            type="file" 
            accept="application/pdf"
            className={styles.fileInput}
            ref={fileInputRef}
            onChange={handleFileChange}
          />
        </div>

        {file && (
          <div className={styles.fileInfo}>
            <span className={styles.fileName}>{file.name}</span>
            <button 
              className={styles.removeButton}
              onClick={() => {
                setFile(null);
                setResult(null);
              }}
            >
              Remove
            </button>
          </div>
        )}

        {error && (
          <div className={styles.error}>
            {error}
          </div>
        )}

        {result && (
          <div className={`${styles.result} ${result.result === "match" ? styles.match : styles.mismatch}`}>
            <h2>{result.result.toUpperCase()}</h2>
            <p>{result.message}</p>
          </div>
        )}

        <button 
          className={styles.submitButton}
          onClick={handleSubmit}
          disabled={!file || loading}
        >
          {loading ? "Verifying..." : "Submit for Verification"}
        </button>
      </div>
    </div>
  );
}
