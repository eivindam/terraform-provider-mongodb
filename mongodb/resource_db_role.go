package mongodb

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
)

func resourceDatabaseRole() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDatabaseRoleCreate,
		ReadContext:   resourceDatabaseRoleRead,
		UpdateContext: resourceDatabaseRoleUpdate,
		DeleteContext: resourceDatabaseRoleDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"database": {
				Type:     schema.TypeString,
				Optional: true,
				Default: "admin",
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"privilege": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 10,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{

						"db": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"collection": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"actions": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
			"inherited_role": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 2,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"db": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"role": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceDatabaseRoleCreate(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
	var client = i.(*mongo.Client)
	var role = data.Get("name").(string)
	var database = data.Get("database").(string)
	var roleList []Role
	var privileges []PrivilegeDto

	privilege := data.Get("privilege").(*schema.Set).List()
	roles := data.Get("inherited_role").(*schema.Set).List()

	roleMapErr := mapstructure.Decode(roles, &roleList)
	if roleMapErr != nil {
		return diag.Errorf("Error decoding map : %s ", roleMapErr)
	}
	privMapErr := mapstructure.Decode(privilege, &privileges)
	if privMapErr != nil {
		return diag.Errorf("Error decoding map : %s ", privMapErr)
	}


	err := createRole(client, role, roleList, privileges, database)

	if err != nil {
		return diag.Errorf("Could not create the role : %s ", err)
	}
	str := database+"."+role
	hx := hex.EncodeToString([]byte(str))
	data.SetId(hx)
	return resourceDatabaseRoleRead(ctx, data, i)
}

func resourceDatabaseRoleDelete(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
	var client = i.(*mongo.Client)
	var stateId = data.State().ID
	id, errEncoding := hex.DecodeString(stateId)
	if errEncoding != nil {
		return diag.Errorf("ID mismatch %s", errEncoding)
	}

	adminDB := client.Database("admin")
	Users := adminDB.Collection("system.roles")
	_, err := Users.DeleteOne(ctx, bson.M{"_id": string(id) })
	if err != nil {
		return diag.Errorf("%s",err)
	}

	return resourceDatabaseRoleRead(ctx, data, i)

}

func resourceDatabaseRoleUpdate(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
	var client = i.(*mongo.Client)
	var role = data.Get("name").(string)
	var database = data.Get("database").(string)
	var stateId = data.State().ID
	id, errEncoding := hex.DecodeString(stateId)
	if errEncoding != nil {
		return diag.Errorf("ID mismatch %s", errEncoding)
	}
	adminDB := client.Database("admin")
	Users := adminDB.Collection("system.roles")
	_, err := Users.DeleteOne(ctx, bson.M{"_id": string(id) })
	if err != nil {
		return diag.Errorf("%s",err)
	}
	var roleList []Role
	var privileges []PrivilegeDto

	privilege := data.Get("privilege").(*schema.Set).List()
	roles := data.Get("inherited_role").(*schema.Set).List()

	roleMapErr := mapstructure.Decode(roles, &roleList)
	if roleMapErr != nil {
		return diag.Errorf("Error decoding map : %s ", roleMapErr)
	}
	privMapErr := mapstructure.Decode(privilege, &privileges)
	if privMapErr != nil {
		return diag.Errorf("Error decoding map : %s ", privMapErr)
	}

	err2 := createRole(client, role, roleList, privileges, database)

	if err2 != nil {
		return diag.Errorf("Could not create the role  :  %s ", err)
	}
	str := database+"."+role
	hx := hex.EncodeToString([]byte(str))
	data.SetId(hx)


	return resourceDatabaseRoleRead(ctx, data, i)
}

func resourceDatabaseRoleRead(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	var client = i.(*mongo.Client)
	stateID := data.State().ID
	roleName, database , err := resourceDatabaseRoleParseId(stateID)
	if err != nil {
		return diag.Errorf("%s",err)
	}
	result , decodeError := getRole(client,roleName,database)
	if decodeError != nil {
		return diag.Errorf("Error decoding role : %s ", err)
	}
	if len(result.Roles) == 0 {
		return diag.Errorf("Role does not exist")
	}
	inheritedRoles := make([]interface{}, len(result.Roles[0].InheritedRoles))

	for i, s := range result.Roles[0].InheritedRoles {
		inheritedRoles[i] = map[string]interface{}{
			"db": s.Db,
			"role": s.Role,
		}
	}
	data.Set("inherited_role", inheritedRoles)
	privileges := make([]interface{}, len(result.Roles[0].Privileges))

	for i, s := range result.Roles[0].Privileges {
		privileges[i] = map[string]interface{}{
			"db": s.Resource.Db,
			"collection": s.Resource.Collection,
			"actions": s.Actions,
		}
	}
	data.Set("privilege", privileges)

	data.Set("database", database)
	data.Set("name", roleName)

	data.SetId(stateID)
	diags = nil
	return diags
}

func resourceDatabaseRoleParseId(id string) (string, string, error) {
	result , errEncoding := hex.DecodeString(id)

	if errEncoding != nil {
		return "", "", fmt.Errorf("unexpected format of ID Error : %s", errEncoding)
	}
	parts := strings.SplitN(string(result), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%s), expected database.roleName", id)
	}

	database := parts[0]
	roleName := parts[1]

	return roleName , database , nil
}

