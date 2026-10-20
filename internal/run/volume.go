package run

// AcquisitionVolume measures normalized observed result payload, not original
// file size, model tokens, or context retained by the agent. Text and image
// bytes are separate dimensions and are never converted into one another.
type AcquisitionVolume struct {
	TextBytes  int
	ImageBytes int
}

// AcquisitionVolume uses the reconstructed acquisition associations. Missing
// observations contribute no known bytes; they remain missing in frequency
// measurements. Empty and tool-error observations are still received payloads.
func (r Run) AcquisitionVolume() AcquisitionVolume {
	var volume AcquisitionVolume
	for _, pair := range r.AcquisitionAssociations() {
		observation := pair.Observation
		if observation == nil {
			continue
		}
		if observation.Image != nil {
			volume.ImageBytes += observation.Image.ByteLength
		} else {
			volume.TextBytes += len(observation.Text)
		}
	}
	return volume
}
