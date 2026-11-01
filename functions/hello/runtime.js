const http = require("http");
const handler = require("./index.js");

const port = Number(process.env.PORT) || 3000;

const server = http.createServer(async (req, res) => {
  let raw = "";
  req.on("data", (chunk) => {
    raw += chunk;
  });

  req.on("end", async () => {
    let body = raw;
    try {
      body = raw ? JSON.parse(raw) : null;
    } catch (_) {
      body = raw;
    }

    const url = new URL(req.url, `http://${req.headers.host}`);
    const event = {
      method: req.method,
      path: url.pathname,
      headers: req.headers,
      query: Object.fromEntries(url.searchParams),
      body,
    };
    const context = {
      name: process.env.FUNCTION_NAME || "hello",
      tenantId: process.env.TENANT_ID || "default",
      version: process.env.FUNCTION_VERSION || "latest",
      env: process.env,
    };

    try {
      const result = await handler(event, context);
      res.writeHead(result.statusCode || 200, result.headers || {});
      res.end(result.body || "");
    } catch (err) {
      res.writeHead(500, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: err.message }));
    }
  });
});

server.listen(port, () => {
  console.log(`function listening on ${port}`);
});
