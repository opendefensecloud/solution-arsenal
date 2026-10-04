{{- $nginx := index .OCIResources "nginx-image" }}
image:
  repository: {{ $nginx.Host }}/{{ $nginx.Repository }}
  tag: {{ $nginx.Tag }}
