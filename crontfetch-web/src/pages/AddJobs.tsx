import { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { addCompany, updateCompany } from '../store/companies/companiesThunks';

export default function AddJobs() {
  const navigate = useNavigate();
  const location = useLocation();
  const dispatch = useAppDispatch();
  const { loading, error } = useAppSelector((state) => state.companies);

  // Check if an existing company was passed through router state for editing
  const existingCompany = location.state?.company || null;
  const isEditing = Boolean(existingCompany);

  const [name, setName] = useState('');
  const [careerUrl, setCareerUrl] = useState('');
  const [minExp, setMinExp] = useState('');
  const [maxExp, setMaxExp] = useState('');
  const [roles, setRoles] = useState('');
  const [locations, setLocations] = useState('');
  const [toast, setToast] = useState<string | null>(null);

  // Pre-fill fields if we are editing an existing company
  useEffect(() => {
    if (isEditing && existingCompany) {
      setName(existingCompany.name || '');
      setCareerUrl(existingCompany.career_url || '');
      setMinExp(existingCompany.min_experience_years !== null ? String(existingCompany.min_experience_years) : '');
      setMaxExp(existingCompany.max_experience_years !== null ? String(existingCompany.max_experience_years) : '');
      setRoles(existingCompany.roles ? existingCompany.roles.join(', ') : '');
      setLocations(existingCompany.locations ? existingCompany.locations.join(', ') : '');
    }
  }, [isEditing, existingCompany]);

  useEffect(() => {
    if (toast) {
      const timer = setTimeout(() => {
        setToast(null);
      }, 3000);
      return () => clearTimeout(timer);
    }
  }, [toast]);

  const handleCancel = () => {
    navigate('/dashboard');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const payload = {
      name,
      career_url: careerUrl,
      min_experience_years: minExp !== '' ? Number(minExp) : null,
      max_experience_years: maxExp !== '' ? Number(maxExp) : null,
      roles: roles ? roles.split(',').map((r) => r.trim()).filter(Boolean) : [],
      locations: locations ? locations.split(',').map((l) => l.trim()).filter(Boolean) : [],
    };

    try {
      if (isEditing) {
        // Call PATCH /companies/{id} via updateCompany thunk
        await dispatch(updateCompany({ id: existingCompany.id, payload })).unwrap();
        setToast('Company updated successfully');
      } else {
        // Call POST /companies via addCompany thunk
        await dispatch(addCompany(payload)).unwrap();
        setToast('Company added successfully');
        
        // Clear fields only when adding a new one
        setName('');
        setCareerUrl('');
        setMinExp('');
        setMaxExp('');
        setRoles('');
        setLocations('');
      }

      // Redirect back to dashboard after a brief delay
      setTimeout(() => {
        navigate('/dashboard');
      }, 1000);
    } catch (err) {
      console.error('Failed to save company:', err);
    }
  };

  return (
    <section className="dashboard">
      <div className="container">
        <div className="dashboard__header">
          <h1>{isEditing ? 'Edit Tracked Company' : 'Track New Career Page'}</h1>
          <p>{isEditing ? 'Update your filters and configurations.' : 'Add a new career page to track job postings.'}</p>
        </div>

        {toast && (
          <div className="toast" style={{ marginBottom: '16px', padding: '10px', background: '#dcfce7', color: '#166534', borderRadius: '4px' }}>
            {toast}
          </div>
        )}

        {error && (
          <div className="ac-error-banner" style={{ marginBottom: '16px' }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="ac-form">
          <div className="ac-form-group">
            <label className="ac-form-label">Company Name *</label>
            <input
              type="text"
              required
              className="ac-form-input"
              placeholder="e.g. Mercedes-Benz"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          <div className="ac-form-group">
            <label className="ac-form-label">Career URL *</label>
            <input
              type="url"
              required
              className="ac-form-input"
              placeholder="https://jobs.mercedes-benz.com/"
              value={careerUrl}
              onChange={(e) => setCareerUrl(e.target.value)}
            />
          </div>

          <div className="ac-form-row">
            <div className="ac-form-group">
              <label className="ac-form-label">Min Exp (Years)</label>
              <input
                type="number"
                min="0"
                className="ac-form-input"
                placeholder="2"
                value={minExp}
                onChange={(e) => setMinExp(e.target.value)}
              />
            </div>
            <div className="ac-form-group">
              <label className="ac-form-label">Max Exp (Years)</label>
              <input
                type="number"
                min="0"
                className="ac-form-input"
                placeholder="6"
                value={maxExp}
                onChange={(e) => setMaxExp(e.target.value)}
              />
            </div>
          </div>

          <div className="ac-form-group">
            <label className="ac-form-label">Roles (comma separated)</label>
            <input
              type="text"
              className="ac-form-input"
              placeholder="software engineer, developer"
              value={roles}
              onChange={(e) => setRoles(e.target.value)}
            />
          </div>

          <div className="ac-form-group">
            <label className="ac-form-label">Locations (comma separated)</label>
            <input
              type="text"
              className="ac-form-input"
              placeholder="India, Remote"
              value={locations}
              onChange={(e) => setLocations(e.target.value)}
            />
          </div>

          <div className="ac-button-group">
            <button type="button" className="ac-btn-secondary" onClick={handleCancel}>
              Cancel
            </button>
            <button type="submit" className="ac-btn-primary" disabled={loading}>
              {loading ? 'Saving...' : isEditing ? 'Update Company' : 'Add Company'}
            </button>
          </div>
        </form>
      </div>
    </section>
  );
}