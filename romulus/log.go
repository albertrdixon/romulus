package romulus

import l "github.com/Sirupsen/logrus"

func LogLevel(lvl string) {
	lv, e := l.ParseLevel(lvl)
	if e != nil {
		l.SetLevel(l.InfoLevel)
	} else {
		l.SetLevel(lv)
	}
}

type fieldSet interface {
	fields() map[string]interface{}
}

type fi map[string]interface{}

func (f fi) fields() map[string]interface{} { return f }

func log() *l.Entry { return logf() }
func logf(fs ...fieldSet) *l.Entry {
	lf := l.Fields{}
	for i := range fs {
		for k, v := range fs[i].fields() {
			lf[k] = v
		}
	}
	return l.WithFields(lf)
}
