// The Angular dev server's proxy (make frontend-serve), the developer's stand-in for the Ingress
// <fullname>-backend: /api goes to the backend on :8080 (make run), as the Ingress routes it on an
// installation (docs/adr/0001 D2), and so do the backend's /healthz and /readyz; the dev server
// serves the rest. DASH_DEV_BACKEND names another backend.
const backend = process.env['DASH_DEV_BACKEND'] ?? 'http://localhost:8080';

export default {
  '/api': { target: backend, secure: false },
  '/healthz': { target: backend, secure: false },
  '/readyz': { target: backend, secure: false },
};
