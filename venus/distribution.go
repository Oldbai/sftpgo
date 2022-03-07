package venus

import "fmt"

type distribution struct {
	fileName string `json:"file_name"`
}

func NewDistribution(fileName string) VService {
	return &distribution{fileName: fileName}
}

func (d *distribution) DistributionFile() (bool, error) {
	fmt.Println(d.fileName)
	return false, nil
}
