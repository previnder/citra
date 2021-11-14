create table if not exists folders (
	id int not null,
	images_count int not null default 0,
	total_size bigint not null default 0,
	created_at datetime not null default current_timestamp(),

	primary key(id)
);

