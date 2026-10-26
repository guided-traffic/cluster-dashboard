// Applies the stored colour scheme before the first paint (docs/adr/0005): the preference the
// theme service keeps under dash.theme becomes the data-theme attribute of <html>, which
// tokens.css turns into the page's color-scheme. Storage that throws, or holds anything else,
// means "system". A file loaded in <head>, because the shell's Content-Security-Policy refuses
// inline scripts.
(function () {
  var preference = 'system';
  try {
    var stored = window.localStorage.getItem('dash.theme');
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      preference = stored;
    }
  } catch (e) {
    // A private window or blocked site data: the system's scheme.
  }
  document.documentElement.setAttribute('data-theme', preference);
})();
