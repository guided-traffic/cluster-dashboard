import {
  ApplicationConfig,
  inject,
  provideAppInitializer,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideHttpClient, withFetch } from '@angular/common/http';
import { provideApiConfiguration } from './api/api-configuration';
import { ThemeService } from './core/theme.service';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideHttpClient(withFetch()),
    // The generated paths start with /api/v1; the root URL must not add a second slash.
    provideApiConfiguration(''),
    // The scheme is in effect before the first render; theme-init.js set it before the first paint.
    provideAppInitializer(() => {
      inject(ThemeService);
    }),
  ],
};
