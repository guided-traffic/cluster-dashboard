import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Title } from '@angular/platform-browser';
import { catchError, map, of } from 'rxjs';
import { DashboardService } from './core/dashboard.service';
import { VersionService } from './core/version.service';
import { DashboardPage } from './features/dashboard/dashboard';
import { ThemeControl } from './shared/theme-control/theme-control';

type BackendVersion =
  { state: 'loading' } | { state: 'known'; version: string } | { state: 'unknown' };

/**
 * The shell (docs/adr/0012 D2): the top bar — the mark and the title, the backend's version and the
 * theme control, cells divided by hairlines — over the dashboard.
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

  /** ui.title once the dashboard has answered; Dash until then. */
  protected readonly title = computed(() => {
    const s = this.dashboard.state();
    return s.kind === 'ready' ? s.dashboard.ui.title : 'Dash';
  });

  constructor() {
    const title = inject(Title);
    effect(() => title.setTitle(this.title()));
  }
}
