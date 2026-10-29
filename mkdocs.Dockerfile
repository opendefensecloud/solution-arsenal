FROM squidfunk/mkdocs-material@sha256:dc3b5de76c6e30d18dc0922d4c4a7bed817e5e155f9ff0e594a53eb56e537e64

COPY ./docs/requirements.txt /requirements.txt

RUN pip install -r /requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
