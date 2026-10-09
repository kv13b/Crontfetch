import { useState } from 'react';
// import { useAddCompanyMutation } from '../services/api';

export default function AddJobs({ isOpen = true, onClose = () => {} }) {
  const [name, setName] = useState('');
  const [careerUrl, setCareerUrl] = useState('');
  const [minExp, setMinExp] = useState('');
  const [maxExp, setMaxExp] = useState('');
  const [roles, setRoles] = useState('');
  const [locations, setLocations] = useState('');

  // --- Redux Mutation Hook Commented Out ---
  // const [addCompany, { isLoading, error }] = useAddCompanyMutation();
  const isLoading = false;
  const error = null;

  if (!isOpen) return null;

  const handleCancel = () => {
    // Reset form state
    setName('');
    setCareerUrl('');
    setMinExp('');
    setMaxExp('');
    setRoles('');
    setLocations('');
    
    // Trigger parent close / navigation handler
    onClose();
  };

  const handleSubmit = (e:any) => {
    e.preventDefault();

    const payload = {
      name,
      career_url: careerUrl,
      min_experience_years: minExp !== '' ? Number(minExp) : null,
      max_experience_years: maxExp !== '' ? Number(maxExp) : null,
      roles: roles ? roles.split(',').map((r) => r.trim()).filter(Boolean) : [],
      locations: locations ? locations.split(',').map((l) => l.trim()).filter(Boolean) : [],
    };

    console.log('UI Form Submitted Payload:', payload);

    // --- Redux API Call Commented Out ---
    // try {
    //   await addCompany(payload).unwrap();
    //   onClose();
    // } catch (err) {
    //   console.error('Failed to add company:', err);
    // }

    onClose();
  };

  return (
    <div className="ac-form-container">
      <h2 className="ac-modal-title">Track New Career Page</h2>

      {error && (
        <div className="ac-error-banner">
          An error occurred
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
          <button type="submit" className="ac-btn-primary" disabled={isLoading}>
            {isLoading ? 'Saving...' : 'Add Company'}
          </button>
        </div>
      </form>
    </div>
  );
}