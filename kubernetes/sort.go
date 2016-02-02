package kubernetes

import "sort"

var (
	ByID  = func(a, b *Resource) bool { return a.id < b.id }
	ByUID = func(a, b *Resource) bool { return a.uid < b.uid }
)

func Sort(list ResourceList, fn func(a, b *Resource) bool) {
	sfn := fn
	if sfn == nil {
		sfn = ByID
	}
	sort.Sort(&resourceListSorter{
		resources: list,
		sorter:    sfn,
	})
}

func (s *resourceListSorter) Len() int {
	return len(s.resources)
}
func (s *resourceListSorter) Swap(i, j int) {
	s.resources[i], s.resources[j] = s.resources[j], s.resources[i]
}
func (s *resourceListSorter) Less(i, j int) bool {
	return s.sorter(s.resources[i], s.resources[j])
}
