module.exports = async function handler(event, context) {
  const name = (event.query && event.query.name) || "world";
  return {
    statusCode: 200,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      message: `Hello ${name}`,
      function: context.name,
      version: context.version,
    }),
  };
};
