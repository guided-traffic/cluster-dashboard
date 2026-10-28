import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideApiConfiguration } from '../api/api-configuration';
import { dashboardOf } from '../../testing/dashboard';
import { DashboardService, retrySeconds } from './dashboard.service';

function setVisibility(state: DocumentVisibilityState): void {
  Object.defineProperty(document, 'visibilityState', { value: state, configurable: true });
  document.dispatchEvent(new Event('visibilitychange'));
}

describe('DashboardService', () => {
  let http: HttpTestingController;
  let service: DashboardService;

  beforeEach(() => {
    vi.useFakeTimers();
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideApiConfiguration('')],
    });
    http = TestBed.inject(HttpTestingController);
    service = TestBed.inject(DashboardService);
  });

  afterEach(() => {
    service.stop();
    http.verify();
    vi.useRealTimers();
  });

  it('asks on start, then every refresh_seconds', () => {
    expect(service.state()).toEqual({ kind: 'loading' });
    service.start();
    service.start();
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    expect(service.state()).toEqual({ kind: 'ready', dashboard: dashboardOf(), stale: false });

    vi.advanceTimersByTime(29_999);
    http.expectNone('/api/v1/dashboard');
    vi.advanceTimersByTime(1);
    http.expectOne('/api/v1/dashboard').flush(dashboardOf({ groups: [] }));
    expect(service.state()).toMatchObject({
      kind: 'ready',
      stale: false,
      dashboard: { groups: [] },
    });
  });

  it('keeps the last answer when a refresh fails, and marks it stale', () => {
    service.start();
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    vi.advanceTimersByTime(30_000);
    http.expectOne('/api/v1/dashboard').flush(null, { status: 502, statusText: 'Bad Gateway' });
    expect(service.state()).toEqual({ kind: 'ready', dashboard: dashboardOf(), stale: true });
    vi.advanceTimersByTime(30_000);
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    expect(service.state()).toMatchObject({ kind: 'ready', stale: false });
  });

  it('tells a starting backend from an unreachable one, and retries soon', () => {
    service.start();
    http
      .expectOne('/api/v1/dashboard')
      .flush(
        { code: 'not_ready', status: 503 },
        { status: 503, statusText: 'Service Unavailable' },
      );
    expect(service.state()).toEqual({ kind: 'failed', reason: 'starting' });
    vi.advanceTimersByTime(retrySeconds * 1000);
    http.expectOne('/api/v1/dashboard').error(new ProgressEvent('error'));
    expect(service.state()).toEqual({ kind: 'failed', reason: 'unreachable' });
    vi.advanceTimersByTime(retrySeconds * 1000);
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    expect(service.state()).toMatchObject({ kind: 'ready' });
  });

  it('asks nothing while the page is hidden, and at once when it is visible again', () => {
    service.start();
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    setVisibility('hidden');
    vi.advanceTimersByTime(300_000);
    http.expectNone('/api/v1/dashboard');
    setVisibility('visible');
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    vi.advanceTimersByTime(30_000);
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
  });

  it('schedules nothing for an answer that arrives while hidden', () => {
    service.start();
    setVisibility('hidden');
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    vi.advanceTimersByTime(300_000);
    http.expectNone('/api/v1/dashboard');
  });

  it('stops asking when stopped', () => {
    service.start();
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
    service.stop();
    vi.advanceTimersByTime(300_000);
    setVisibility('visible');
    http.expectNone('/api/v1/dashboard');
  });

  it('drops a question still running when it asks again', () => {
    service.start();
    const first = http.expectOne('/api/v1/dashboard');
    service.refresh();
    expect(first.cancelled).toBe(true);
    http.expectOne('/api/v1/dashboard').flush(dashboardOf());
  });
});
