{{- $img := index .OCIResources "solar-agent-image" }}
image:
  repository: {{ $img.Host }}/{{ $img.Repository }}
  tag: {{ $img.Tag }}
{{- with pullSecretFor (printf "%s/%s" $img.Host $img.Repository) }}
imagePullSecrets:
  - name: {{ . }}
{{- end }}
