{{- /* One template for the whole umbrella: every sub-chart's image keys are
kept here by hand, under the sub-chart's name. A key that is missing or
misspelt is not an error; that sub-chart keeps pulling from upstream. */ -}}
{{- $pause := index .OCIResources "pause-image" }}
{{- $kubectl := index .OCIResources "kubectl-image" }}
widget-operator:
  image:
    repository: '{{ $kubectl.Host }}/{{ $kubectl.Repository }}'
    tag: '{{ $kubectl.Tag }}'
widget-instance:
  image:
    repository: '{{ $pause.Host }}/{{ $pause.Repository }}'
    tag: '{{ $pause.Tag }}'
