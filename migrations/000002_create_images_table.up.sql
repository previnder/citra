create table if not exists images (
	id binary (12) not null,
	folder_id int not null,
	`type` varchar (24) not null,
	width int not null,
	height int not null,
	max_width int not null,
	max_height int not null,
	`size` int not null,
	uploaded_size int not null,
	average_color varchar (32) not null,
	copies JSON,
	created_at datetime not null default current_timestamp(),
	is_deleted bool not null default false,
	deleted_at datetime,

	foreign key (folder_id) references folders (id),
	primary key (id)
);
