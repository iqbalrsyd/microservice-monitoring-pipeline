FROM prom/prometheus:v2.47.2

# Copy configuration files with proper ownership
COPY --chown=prometheus:prometheus prometheus.yml /etc/prometheus/
COPY --chown=prometheus:prometheus rules/ /etc/prometheus/rules/

# Expose port
EXPOSE 9090

# Command to run Prometheus with custom config
CMD ["--config.file=/etc/prometheus/prometheus.yml", \
     "--storage.tsdb.path=/prometheus", \
     "--web.console.libraries=/etc/prometheus/console_libraries", \
     "--web.console.templates=/etc/prometheus/consoles", \
     "--storage.tsdb.retention.time=30d", \
     "--web.enable-lifecycle"]