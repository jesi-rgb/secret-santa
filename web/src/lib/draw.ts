// Client-side mirror of the backend's draw feasibility check (draw.go).

/**
 * Whether a draw exists where nobody gifts themselves or someone they are
 * excluded with. `pairs` holds participant indices; each pair applies both ways.
 */
export function canDraw(count: number, pairs: readonly (readonly [number, number])[]): boolean {
  const excluded = Array.from({ length: count }, () => new Set<number>());
  for (const [a, b] of pairs) {
    excluded[a].add(b);
    excluded[b].add(a);
  }
  // Bipartite matching by augmenting paths (Kuhn's algorithm).
  const giverOf = new Array<number>(count).fill(-1);
  const visited = new Array<boolean>(count);
  const augment = (giver: number): boolean => {
    for (let giftee = 0; giftee < count; giftee++) {
      if (giftee === giver || excluded[giver].has(giftee) || visited[giftee]) continue;
      visited[giftee] = true;
      if (giverOf[giftee] < 0 || augment(giverOf[giftee])) {
        giverOf[giftee] = giver;
        return true;
      }
    }
    return false;
  };
  for (let giver = 0; giver < count; giver++) {
    visited.fill(false);
    if (!augment(giver)) return false;
  }
  return true;
}
