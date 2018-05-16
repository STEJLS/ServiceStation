CREATE TABLE users (
id serial PRIMARY KEY,
login varchar (25) NOT NULL UNIQUE,
password varchar (50) NOT NULL,
name varchar(50),
lastName varchar (50) NOT NULL,
phone varchar (20) NOT NULL,
profileImage boolean NOT NULL 
);


CREATE TABLE cars (
id serial PRIMARY KEY,
brand varchar (20) NOT NULL,
model varchar (50) NOT NULL,
vin varchar(17),
year varchar (4) NOT NULL,
userid integer REFERENCES users(id),
deleted boolean NOT NULL DEFAULT FALSE
);


CREATE TABLE orders (
id serial PRIMARY KEY,
status smallint NOT NULL,
date Date NOT NULL, 
cost integer,
carID integer REFERENCES cars(id),
userID integer REFERENCES users(id),
info varchar NOT NULL,
newmsgforuser boolean NOT NULL DEFAULT FALSE
);


CREATE TABLE authorizations(
userid integer REFERENCES users(id),
token char(36) NOT NULL UNIQUE
);

CREATE TABLE messages (
isadmin boolean NOT NULL DEFAULT FALSE,
date timestamp  NOT NULL, 
text varchar NOT NULL,
orderID integer REFERENCES orders(id)
);