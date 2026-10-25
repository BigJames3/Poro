// Poro-Capabilities (ADR-0020): what a client can display, as comma-separated
// case-insensitive tokens; unknown tokens are ignored. Without the header, as
// from every app released before photo posts, a client declares nothing.

/** Lower case: Node lowercases incoming header names. */
export const CAPABILITIES_HEADER = 'poro-capabilities';

export const CAPABILITY_PHOTO_POSTS = 'photo-posts';

export function hasCapability(header: string | undefined, token: string): boolean {
  if (header === undefined) {
    return false;
  }
  const wanted = token.toLowerCase();
  return header.split(',').some((part) => part.trim().toLowerCase() === wanted);
}
