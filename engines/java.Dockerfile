FROM eclipse-temurin:21-jdk AS build
ARG REPO
ARG COMMIT
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone "$REPO" app && cd app && git checkout "$COMMIT"
# o Simples não tem flyway/; usa o do regular@150cccb
RUN git clone https://github.com/ReceitaFederal/calculadora-rtc-regime-regular.git reg \
 && git -C reg checkout 150cccb && cp -r reg/flyway app/flyway
WORKDIR /src/app
RUN mkdir -p calculadora/db && ./mvnw -B flyway:migrate -Dflyway.configFiles=./flyway/flyway-pro.conf
RUN ./mvnw -B -DskipTests package

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /src/app/target/*.jar app.jar
COPY --from=build /src/app/calculadora/db/calculadora-pro.db calculadora/db/calculadora-pro.db
ENV SPRING_PROFILES_ACTIVE=offline
ENTRYPOINT ["java","-XX:MaxRAMPercentage=75","-jar","/app/app.jar"]
