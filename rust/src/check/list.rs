//! Resolving a list of named types (F17 12.2): the uses between its items,
//! which items are recursive, the cycles with no guard, and the order in
//! which the components are resolved.
//!
//! The walks keep their own stacks: a list may hold thousands of items.

/// The uses of each item: the item named, and whether the name stands at a
/// top place (12.1).
pub struct Graph {
    pub uses: Vec<Vec<(usize, bool)>>,
}

/// How a list is resolved.
pub struct Plan {
    /// The components, each after every component it names (12.2 item 4).
    pub components: Vec<Vec<usize>>,
    /// Whether each item is recursive: on a cycle of uses.
    pub recursive: Vec<bool>,
    /// Whether each item is on a cycle of uses that all stand at top places
    /// (12.2 item 3).
    pub direct: Vec<bool>,
    /// For each item, its place in an order in which each item comes after
    /// the items it names at top places, where those uses form no cycle.
    pub top_rank: Vec<usize>,
}

/// The strongly connected components of a graph, each after every
/// component it reaches (Tarjan's order), by an explicit stack.
fn components(n: usize, succ: &dyn Fn(usize) -> Vec<usize>) -> Vec<Vec<usize>> {
    const NONE: usize = usize::MAX;
    let mut index = vec![NONE; n];
    let mut low = vec![0usize; n];
    let mut on = vec![false; n];
    let mut stack: Vec<usize> = Vec::new();
    let mut out: Vec<Vec<usize>> = Vec::new();
    let mut next = 0usize;
    for s in 0..n {
        if index[s] != NONE {
            continue;
        }
        let mut call: Vec<(usize, Vec<usize>, usize)> = Vec::new();
        index[s] = next;
        low[s] = next;
        next += 1;
        stack.push(s);
        on[s] = true;
        call.push((s, succ(s), 0));
        while let Some((v, ws, i)) = call.last_mut() {
            let v = *v;
            if *i < ws.len() {
                let w = ws[*i];
                *i += 1;
                if index[w] == NONE {
                    index[w] = next;
                    low[w] = next;
                    next += 1;
                    stack.push(w);
                    on[w] = true;
                    let sw = succ(w);
                    call.push((w, sw, 0));
                } else if on[w] {
                    low[v] = low[v].min(index[w]);
                }
            } else {
                call.pop();
                if let Some((u, _, _)) = call.last() {
                    low[*u] = low[*u].min(low[v]);
                }
                if low[v] == index[v] {
                    let mut c = Vec::new();
                    while let Some(w) = stack.pop() {
                        on[w] = false;
                        c.push(w);
                        if w == v {
                            break;
                        }
                    }
                    c.sort_unstable();
                    out.push(c);
                }
            }
        }
    }
    out
}

/// Plan the resolution of a list.
pub fn plan(g: &Graph) -> Plan {
    let n = g.uses.len();
    let all = |i: usize| -> Vec<usize> { g.uses[i].iter().map(|(j, _)| *j).collect() };
    let comps = components(n, &all);
    let mut recursive = vec![false; n];
    for c in &comps {
        let cyclic = c.len() > 1 || g.uses[c[0]].iter().any(|(j, _)| *j == c[0]);
        for &i in c {
            recursive[i] = cyclic;
        }
    }
    let top = |i: usize| -> Vec<usize> {
        g.uses[i]
            .iter()
            .filter(|(_, t)| *t)
            .map(|(j, _)| *j)
            .collect()
    };
    let tops = components(n, &top);
    let mut direct = vec![false; n];
    let mut top_rank = vec![0usize; n];
    for (r, c) in tops.iter().enumerate() {
        let cyclic = c.len() > 1 || g.uses[c[0]].iter().any(|(j, t)| *t && *j == c[0]);
        for &i in c {
            direct[i] = cyclic;
            top_rank[i] = r;
        }
    }
    Plan {
        components: comps,
        recursive,
        direct,
        top_rank,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn components_come_after_those_they_name() {
        // 0 -> 1 (top), 1 -> 2 (guarded), 2 -> 1 (top), 3 -> 3 (top)
        let g = Graph {
            uses: vec![
                vec![(1, true)],
                vec![(2, false)],
                vec![(1, true)],
                vec![(3, true)],
            ],
        };
        let p = plan(&g);
        let pos = |i: usize| p.components.iter().position(|c| c.contains(&i)).unwrap();
        assert!(pos(1) < pos(0));
        assert_eq!(pos(1), pos(2));
        assert_eq!(p.recursive, vec![false, true, true, true]);
        assert_eq!(p.direct, vec![false, false, false, true]);
        assert!(p.top_rank[1] < p.top_rank[2]);
    }

    #[test]
    fn a_long_chain_needs_no_recursion() {
        let n = 100_000;
        let g = Graph {
            uses: (0..n).map(|i| vec![((i + 1) % n, false)]).collect(),
        };
        let p = plan(&g);
        assert_eq!(p.components.len(), 1);
        assert!(p.recursive.iter().all(|r| *r));
    }
}
