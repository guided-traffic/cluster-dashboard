import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Title } from '@angular/platform-browser';
import { catchError, map, of } from 'rxjs';
import { Clock, greeting, longDate } from './core/clock';
import { DashboardService } from './core/dashboard.service';
import { VersionService } from './core/version.service';
import { DashboardPage } from './features/dashboard/dashboard';
import { ThemeControl } from './shared/theme-control/theme-control';

type BackendVersion =
  { state: 'loading' } | { state: 'known'; version: string } | { state: 'unknown' };

/**
 * The shell: the top bar — chrome, darker than the stage — with the title, the greeting and the
 * date in muted text, the backend's version and the theme control; under it the dashboard.
 */
@Component({
  selector: 'app-root',
  imports: [ThemeControl, DashboardPage],
  templateUrl: './app.html',
  styleUrl: './app.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  private readonly versions = inject(VersionService);
  private readonly dashboard = inject(DashboardService);
  private readonly clock = inject(Clock);

  protected readonly backend = toSignal(
    this.versions.get().pipe(
      map((v): BackendVersion => ({ state: 'known', version: v.version })),
      catchError(() => of<BackendVersion>({ state: 'unknown' })),
    ),
    { initialValue: { state: 'loading' } as BackendVersion },
  );

  protected readonly versionText = computed(() => {
    const backend = this.backend();
    switch (backend.state) {
      case 'known':
        return backend.version;
      case 'unknown':
        return 'backend unreachable';
      default:
        return '';
    }
  });

  private readonly ui = computed(() => {
    const s = this.dashboard.state();
    return s.kind === 'ready' ? s.dashboard.ui : undefined;
  });

  /** ui.title once the dashboard has answered; Dash until then. */
  protected readonly title = computed(() => this.ui()?.title ?? 'Dash');

  /** "Good evening, You · Monday 26 October", while ui.greeting is enabled. */
  protected readonly greetingLine = computed(() => {
    const ui = this.ui();
    if (!ui?.greeting.enabled) {
      return '';
    }
    const now = this.clock.now();
    return `${greeting(now, ui.greeting.name)} · ${longDate(now)}`;
  });

  constructor() {
    const title = inject(Title);
    effect(() => title.setTitle(this.title()));
  }
}
