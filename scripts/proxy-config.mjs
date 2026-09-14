// Shared secret parsing and curl configuration; no network or logging.
export function proxyConfiguration(value) {
  if (
    typeof value !== "string" ||
    value !== value.trim() ||
    /[\r\n\0]/.test(value)
  )
    throw Error("Invalid proxy configuration");
  const url = new URL(value);
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  )
    throw Error("Invalid proxy configuration");
  const username = decodeURIComponent(url.username),
    password = decodeURIComponent(url.password);
  if (
    [...(username + password)].some(
      (char) => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127,
    )
  )
    throw Error("Invalid proxy configuration");
  return {
    server: url.origin,
    hostname: url.hostname.replace(/^\[|\]$/g, ""),
    protocol: url.protocol,
    username,
    password,
  };
}

export function cleanEnvironment() {
  const env = { ...process.env };
  for (const key of Object.keys(env))
    if (
      /proxy/i.test(key) ||
      [
        "NODE_DEBUG",
        "DEBUG",
        "SSLKEYLOGFILE",
        "CURL_HOME",
        "CURL_CA_BUNDLE",
        "SSL_CERT_FILE",
        "SSL_CERT_DIR",
        "NODE_EXTRA_CA_CERTS",
        "NODE_TLS_REJECT_UNAUTHORIZED",
      ].includes(key)
    )
      delete env[key];
  return env;
}

// curl config quoting, not shell escaping. Pass this over stdin only.
export const quote = (value) =>
  '"' + value.replaceAll("\\", "\\\\").replaceAll('"', '\\"') + '"';
