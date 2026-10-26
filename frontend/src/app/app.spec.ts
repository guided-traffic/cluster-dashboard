import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { dashboardOf } from '../testing/dashboard';
import { provideApiConfiguration } from './api/api-configuration';
import { App } from './app';
import { DashboardService } from './core/dashboard.service';

describe('App', () => {
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideApiConfiguration('')],
    });
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    TestBed.inject(DashboardService).stop();
    http.verify();
    document.documentElement.removeAttribute('data-theme');
    localStorage.clear();
  });

  function render() {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    // The dashboard asks too; these tests are the shell's.
    http.expectOne('/api/v1/dashboard');
    const root = fixture.nativeElement as HTMLElement;
    const version = () => root.querySelector<HTMLElement>('[data-testid=backend-version]')!;
    return { fixture, root, version };
  }

  it('shows the title and the theme control', () => {
    const { root } = render();
    http.expectOne('/api/v1/version');
    expect(root.querySelector('h1')?.textContent?.trim()).toBe('Dash');
    expect(root.querySelector('app-theme-control')).not.toBeNull();
  });

  it('shows nothing for the version while it loads', () => {
    const { version } = render();
    http.expectOne('/api/v1/version');
    expect(version().textContent?.trim()).toBe('');
  });

  it('shows the backend version once it answers', async () => {
    const { fixture, version } = render();
    http
      .expectOne('/api/v1/version')
      .flush({ version: '0.1.0', commit: 'abc1234', build_time: '0' });
    await fixture.whenStable();
    expect(version().textContent?.trim()).toBe('0.1.0');
    expect(version().getAttribute('aria-label')).toBe('backend version 0.1.0');
    expect(version().classList.contains('unknown')).toBe(false);
  });

  it('says the backend is unreachable when the request fails', async () => {
    const { fixture, version } = render();
    http.expectOne('/api/v1/version').flush(null, { status: 502, statusText: 'Bad Gateway' });
    await fixture.whenStable();
    expect(version().textContent?.trim()).toBe('backend unreachable');
    expect(version().classList.contains('unknown')).toBe(true);
    expect(version().getAttribute('aria-label')).toBeNull();
  });

  it('takes the title and the greeting from the dashboard', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 26, 19, 30));
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    http
      .expectOne('/api/v1/version')
      .flush({ version: '0.1.0', commit: 'abc1234', build_time: '0' });
    const d = dashboardOf();
    d.ui = { ...d.ui, title: 'Home lab', greeting: { enabled: true, name: 'Hans' } };
    http.expectOne('/api/v1/dashboard').flush(d);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('h1')!.textContent!.trim()).toBe('Home lab');
    expect(document.title).toBe('Home lab');
    expect(root.querySelector('[data-testid=greeting]')!.textContent!.trim()).toMatch(
      /^Good evening, Hans · /,
    );
    vi.useRealTimers();
  });

  it('greets nobody while ui.greeting is off', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    http.expectOne('/api/v1/version');
    const d = dashboardOf();
    d.ui = { ...d.ui, greeting: { enabled: false, name: 'Hans' } };
    http.expectOne('/api/v1/dashboard').flush(d);
    fixture.detectChanges();
    expect(
      (fixture.nativeElement as HTMLElement).querySelector('[data-testid=greeting]'),
    ).toBeNull();
  });
});
