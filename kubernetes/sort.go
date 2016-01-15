package kubernetes

import "sort"

func Sort(res ResourceList, fn func(a, b *Resource) bool) {
	sfn := fn
	if sfn == nil {
		sfn = func(a, b *Resource) bool {
			return a.id < b.id
		}
	}
	sort.Sort(&Sorter{
		resources: res,
		sorter:    sfn,
	})
}

func (s *Sorter) Len() int {
	return len(s.resources)
}
func (s *Sorter) Swap(i, j int) {
	s.resources[i], s.resources[j] = s.resources[j], s.resources[i]
}
func (s *Sorter) Less(i, j int) bool {
	return s.sorter(s.resources[i], s.resources[j])
}
