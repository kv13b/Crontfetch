import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAppDispatch, useAppSelector } from "../store/hooks";
import {
  deleteCompany,
  fetchCompanies,
} from "../store/companies/companiesThunks";

function DashboardPage() {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const { loading, error, companies } = useAppSelector((s) => s.companies);

  useEffect(() => {
    dispatch(fetchCompanies());
  }, [dispatch]);

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
          {companies.length > 0 && (
            <button
              type="button"
              className="btn btn--primary"
              onClick={() => navigate("/add-jobs")}
            >
              + Add another company
            </button>
          )}
        </div>

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
                <div className="company-info">
                  <h3>{company.name}</h3>
                  <a
                    href={company.career_url}
                    target="_blank"
                    rel="noreferrer"
                    className="company-url"
                  >
                    🔗 {getDomain(company.career_url)}
                  </a>
                </div>
                <div className="company-actions">
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
                  <a
                    href={company.career_url}
                    target="_blank"
                    rel="noreferrer"
                    className="btn btn--pricompaniesmary btn--sm"
                    style={{ textDecoration: "none", textAlign: "center" }}
                  >
                    Career Page ↗
                  </a>
                  <button
                    type="button"
                    className="btn btn--danger btn--sm"
                    style={{
                      background: "#fee2e2",
                      color: "#991b1b",
                      border: "none",
                    }}
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
