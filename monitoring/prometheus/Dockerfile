FROM prom/prometheus:v2.47.2

# Copy configuration files
COPY prometheus.yml /etc/prometheus/
COPY rules/ /etc/prometheus/rules/

# Set permissions
RUN chown -R prometheus:prometheus /etc/prometheus

# Expose port
EXPOSE 9090

# Command to run Prometheus with custom config
CMD ["--config.file=/etc/prometheus/prometheus.yml", \
     "--storage.tsdb.path=/prometheus", \
     "--web.console.libraries=/etc/prometheus/console_libraries", \
     "--web.console.templates=/etc/prometheus/consoles", \
     "--storage.tsdb.retention.time=30d", \
     "--web.enable-lifecycle"]