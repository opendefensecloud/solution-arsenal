FROM squidfunk/mkdocs-material@sha256:fd13d78a4b1da637405c0336c9f0a4e0594b59271df106bd9cd64cae56cd1f38

COPY ./docs/requirements.txt /requirements.txt

RUN pip install -r /requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
