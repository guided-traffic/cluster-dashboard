import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { catchError, map, of } from 'rxjs';
import { VersionService } from './core/version.service';
import { ThemeControl } from './shared/theme-control/theme-control';

type BackendVersion = { state: 'loading' } | { state: 'known'; version: string } | { state: 'unknown' };

/**
 * The shell: the top bar with the title, the backend's version and the theme control, over the
 * stage the dashboard will fill.
 */
@Component({
  selector: 'app-root',
  imports: [ThemeControl],
  templateUrl: './app.html',
  styleUrl: './app.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  private readonly versions = inject(VersionService);

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
}
