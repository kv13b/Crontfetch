import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchPreferences } from "../services/profileService";
import { runSyncNow } from "../services/syncService";
import type { SyncResult } from "../services/syncService";
import { useAppDispatch, useAppSelector } from "../store/hooks";
import {
  deleteCompany,
  fetchCompanies,
} from "../store/companies/companiesThunks";
import { filterFromProfile, isFilterEmpty } from "../types/filter";

// Show a few chips, then "+N" so long filter lists don't stretch the card.
const MAX_CHIPS = 3;

function FilterRow({ label, values }: { label: string; values: string[] }) {
  return (
    <div className="company-card__filter">
      <dt>{label}</dt>
      <dd>
        {values.length === 0 ? (
          <span className="company-card__any">Any</span>
        ) : (
          <>
            {values.slice(0, MAX_CHIPS).map((v) => (
              <span key={v} className="company-card__chip">
                {v}
              </span>
            ))}
            {values.length > MAX_CHIPS && (
              <span className="company-card__more">
                +{values.length - MAX_CHIPS}
              </span>
            )}
          </>
        )}
      </dd>
    </div>
  );
}

function experienceText(min: number | null, max: number | null): string {
  if (min !== null && max !== null) return `${min}–${max} yrs`;
  if (min !== null) return `${min}+ yrs`;
  if (max !== null) return `Up to ${max} yrs`;
  return "Any";
}

function syncSummary(r: SyncResult): { ok: boolean; text: string } {
  const jobs = `${r.new_jobs} new ${r.new_jobs === 1 ? "job" : "jobs"}`;
  const text = `Checked ${r.companies} ${r.companies === 1 ? "company" : "companies"}: ${jobs}${r.new_jobs > 0 ? ", sent to Telegram" : ""}.`;
  if (r.failed.length === 0) return { ok: true, text };
  return {
    ok: false,
    text: `${text} Couldn't check: ${r.failed.map((f) => f.name).join(", ")}.`,
  };
}

function DashboardPage() {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const { loading, error, companies } = useAppSelector((s) => s.companies);
  const token = useAppSelector((s) => s.auth.token);
  const [hasPreferences, setHasPreferences] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [syncResult, setSyncResult] = useState<{
    ok: boolean;
    text: string;
  } | null>(null);

  // The result message fades out on its own.
  useEffect(() => {
    if (!syncResult) return;
    const timer = setTimeout(() => setSyncResult(null), 8000);
    return () => clearTimeout(timer);
  }, [syncResult]);

  const syncNow = async () => {
    setSyncing(true);
    setSyncResult(null);
    try {
      setSyncResult(syncSummary(await runSyncNow(token)));
    } catch (e) {
      setSyncResult({
        ok: false,
        text: e instanceof Error ? e.message : "Sync failed",
      });
    } finally {
      setSyncing(false);
    }
  };

  useEffect(() => {
    dispatch(fetchCompanies());
  }, [dispatch]);

  // Only changes the button label, so a failed load just keeps "Set".
  useEffect(() => {
    let cancelled = false;
    fetchPreferences(token)
      .then((p) => {
        if (!cancelled) setHasPreferences(!isFilterEmpty(filterFromProfile(p)));
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [token]);

  const getDomain = (url: string) => {
    try {
      return new URL(url).hostname;
    } catch {
      return url;
    }
  };
  if (loading) {
    return (
      <section className="dashboard">
        <div className="container">
          <div className="dashboard__header">
            <h1>Welcome</h1>
            <p>Loading your companies...</p>
          </div>
        </div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="dashboard">
        <div className="container">
          <div className="dashboard__header">
            <h1>Welcome</h1>
            <p>Error loading companies.</p>
          </div>
          <div className="card">
            <h2>Error</h2>
            <p>{error}</p>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="dashboard">
      <div className="container">
        {/* Header Section */}
        <div className="dashboard__top-bar">
          <div>
            <h1>
              Welcome
              {companies.length > 0
                ? `, you have ${companies.length} company tracked`
                : ""}{" "}
              👋
            </h1>
            <p>
              {companies.length === 0
                ? "No companies tracked yet. Add a career page to start getting alerts."
                : "Here is where your tracked companies and live job updates appear."}
            </p>
          </div>
          <div className="dashboard__actions">
            <button
              type="button"
              className="btn btn--secondary"
              onClick={() => navigate("/profile")}
            >
              {hasPreferences ? "Update job preferences" : "Set job preferences"}
            </button>
            {companies.length > 0 && (
              <>
                <button
                  type="button"
                  className="btn btn--secondary"
                  onClick={syncNow}
                  disabled={syncing}
                >
                  {syncing ? "Syncing…" : "Sync now"}
                </button>
                <button
                  type="button"
                  className="btn btn--primary"
                  onClick={() => navigate("/add-jobs")}
                >
                  + Add another company
                </button>
              </>
            )}
          </div>
        </div>

        {(syncing || syncResult) && (
          <div className="dashboard__notice">
            {syncing ? (
              <p className="alert alert--info" role="status">
                Checking your companies for new jobs. This can take up to a
                minute.
              </p>
            ) : (
              syncResult && (
                <p
                  className={`alert ${syncResult.ok ? "alert--success" : "alert--error"}`}
                  role={syncResult.ok ? "status" : "alert"}
                >
                  {syncResult.text}
                </p>
              )
            )}
          </div>
        )}

        {/* Content Section */}
        {companies.length === 0 ? (
          <div className="card empty-state">
            <h2>No companies tracked yet</h2>
            <p>
              Add a career page to start getting alerts for roles that match
              you.
            </p>
            <button
              type="button"
              className="btn btn--primary"
              onClick={() => navigate("/add-jobs")}
            >
              Add company
            </button>
          </div>
        ) : (
          <div className="company-grid">
            {companies.map((company) => (
              <div key={company.id} className="card company-card">
                <div className="company-card__head">
                  <span className="company-card__avatar" aria-hidden="true">
                    {company.name.trim().charAt(0).toUpperCase()}
                  </span>
                  <div className="company-info">
                    <h3>{company.name}</h3>
                    <a
                      href={company.career_url}
                      target="_blank"
                      rel="noreferrer"
                      className="company-url"
                    >
                      {getDomain(company.career_url)} ↗
                    </a>
                  </div>
                  {company.platform && (
                    <span className="badge company-card__platform">
                      {company.platform}
                    </span>
                  )}
                </div>

                <dl className="company-card__filters">
                  <FilterRow label="Roles" values={company.roles} />
                  <FilterRow label="Locations" values={company.locations} />
                  <div className="company-card__filter">
                    <dt>Experience</dt>
                    <dd>
                      {experienceText(
                        company.min_experience_years,
                        company.max_experience_years,
                      )}
                    </dd>
                  </div>
                </dl>

                <div className="company-actions">
                  <button
                    type="button"
                    className="btn btn--primary btn--sm company-actions__main"
                    onClick={() => navigate(`/companies/${company.id}/jobs`)}
                  >
                    View jobs
                  </button>
                  {/* Edit button passes company object in location state */}
                  <button
                    type="button"
                    className="btn btn--secondary btn--sm"
                    onClick={() =>
                      navigate("/add-jobs", { state: { company } })
                    }
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    className="btn btn--danger btn--sm"
                    onClick={() => {
                      if (
                        window.confirm(
                          `Are you sure you want to stop tracking ${company.name}?`,
                        )
                      ) {
                        dispatch(deleteCompany(company.id));
                      }
                    }}
                  >
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

export default DashboardPage;
