FROM squidfunk/mkdocs-material@sha256:51b87149d227691486b5f08993d28c65ca7e4990010664b697265b8e6fcd5287

COPY ./docs/requirements.txt /requirements.txt

RUN pip install -r /requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
