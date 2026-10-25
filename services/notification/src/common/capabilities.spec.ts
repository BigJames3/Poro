import { CAPABILITY_PHOTO_POSTS, hasCapability } from './capabilities';

describe('hasCapability', () => {
  it.each([
    [undefined, false],
    ['', false],
    ['photo-posts', true],
    ['Photo-Posts', true],
    [' reels , photo-posts ', true],
    ['photo-posts-v2', false],
    ['photo', false],
  ])('%p -> %p', (header, expected) => {
    expect(hasCapability(header, CAPABILITY_PHOTO_POSTS)).toBe(expected);
  });
});
