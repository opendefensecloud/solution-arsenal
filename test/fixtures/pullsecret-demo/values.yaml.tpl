{{- $nginx := index .OCIResources "nginx-image" }}
image:
  repository: {{ $nginx.Host }}/{{ $nginx.Repository }}
  tag: {{ $nginx.Tag }}
{{- with pullSecretFor (printf "%s/%s" $nginx.Host $nginx.Repository) }}
imagePullSecrets:
  - name: {{ . }}
{{- end }}
