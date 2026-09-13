// Shared bounded transport for public HTML captures. Parsing stays provider-specific.
export async function readHTML(url, fetcher = fetch) {
  const response = await fetcher(url, {
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (
    !response.ok ||
    !response.headers.get("content-type")?.startsWith("text/html")
  )
    throw Error("Calendar HTTP or content-type failure");
  const reader = response.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 1024 * 1024) throw Error("Calendar exceeds 1 MiB");
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
}
