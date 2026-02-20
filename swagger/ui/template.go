package swaggerui

// swaggerUIHTML is the Swagger UI page template.
// Assets (CSS, JS) are loaded from the unpkg CDN so no local files are needed.
// Template variables: .Title, .SpecURL, .CDNVersion
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@{{.CDNVersion}}/swagger-ui.css">
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; background: #fafafa; }
    #swagger-ui { max-width: 1460px; margin: 0 auto; padding: 0 16px; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@{{.CDNVersion}}/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "{{.SpecURL}}",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        plugins: [SwaggerUIBundle.plugins.DownloadUrl],
        layout: "BaseLayout",
        tryItOutEnabled: true
      });
    };
  </script>
</body>
</html>`
