FROM squidfunk/mkdocs-material@sha256:f797da217ff01b20b298d2185eb9d88fb81f5a322c439eba854f2e4de8a48e65

COPY ./docs/requirements.txt /requirements.txt

RUN pip install -r /requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
