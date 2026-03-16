FROM squidfunk/mkdocs-material

RUN pip install mkdocs-glightbox
RUN pip install mkdocs-include-markdown-plugin
RUN pip install mkdocs-panzoom-plugin
RUN pip install mkdocs-awesome-nav

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
