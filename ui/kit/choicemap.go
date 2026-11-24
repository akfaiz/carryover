package kit

// Choice maps Select labels to values.
type Choice struct {
	Labels []string
	Values []string
}

func (c *Choice) Add(value, label string) {
	c.Values = append(c.Values, value)
	c.Labels = append(c.Labels, label)
}

func (c *Choice) Label(v string) string {
	for i, x := range c.Values {
		if x == v {
			return c.Labels[i]
		}
	}
	return ""
}

func (c *Choice) Value(l string) string {
	for i, x := range c.Labels {
		if x == l {
			return c.Values[i]
		}
	}
	return ""
}
