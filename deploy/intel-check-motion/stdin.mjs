export async function readJSONStdin(input = process.stdin) {
  const chunks = []
  for await (const chunk of input) chunks.push(Buffer.from(chunk))
  return JSON.parse(Buffer.concat(chunks).toString('utf8'))
}
