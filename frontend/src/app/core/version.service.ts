import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { map, Observable } from 'rxjs';
import { ApiConfiguration } from '../api/api-configuration';
import { getVersion } from '../api/fn/meta/get-version';
import { Version } from '../api/models/version';

/** The backend's build, GET /api/v1/version through the generated client. */
@Injectable({ providedIn: 'root' })
export class VersionService {
  private readonly http = inject(HttpClient);
  private readonly config = inject(ApiConfiguration);

  get(): Observable<Version> {
    return getVersion(this.http, this.config.rootUrl).pipe(map((response) => response.body));
  }
}
