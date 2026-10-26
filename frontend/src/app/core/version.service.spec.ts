import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';
import { provideApiConfiguration } from '../api/api-configuration';
import { VersionService } from './version.service';

describe('VersionService', () => {
  let http: HttpTestingController;
  let service: VersionService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), provideApiConfiguration('')],
    });
    http = TestBed.inject(HttpTestingController);
    service = TestBed.inject(VersionService);
  });

  afterEach(() => http.verify());

  it('reads the version from GET /api/v1/version', async () => {
    const version = firstValueFrom(service.get());
    const request = http.expectOne('/api/v1/version');
    expect(request.request.method).toBe('GET');
    expect(request.request.headers.get('Accept')).toBe('application/json');
    request.flush({ version: '0.1.0', commit: 'abc1234', build_time: '1793000000' });
    expect(await version).toEqual({ version: '0.1.0', commit: 'abc1234', build_time: '1793000000' });
  });

  it('fails with the error the backend answers', async () => {
    const version = firstValueFrom(service.get());
    http
      .expectOne('/api/v1/version')
      .flush(
        { type: 'about:blank', title: 'Service Unavailable', status: 503, code: 'not_ready' },
        { status: 503, statusText: 'Service Unavailable' },
      );
    await expect(version).rejects.toMatchObject({ status: 503 });
  });
});
